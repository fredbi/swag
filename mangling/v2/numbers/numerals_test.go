package numbers

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

func TestRuneNumber(t *testing.T) {
	t.Parallel()

	cases := map[rune]float64{
		'½': 0.5,
		'¼': 0.25,
		'⅐': 1.0 / 7.0,
		'⅒': 0.1,
		'²': 2,
		'Ⅶ': 7, // roman numeral (Nl)
		'②': 2, // circled digit (No)
	}
	for r, want := range cases {
		v, ok := RuneNumber(r)
		assert.Truef(t, ok, "RuneNumber(%q) should be a numeral", r)
		assert.InDeltaf(t, want, v, 1e-6, "RuneNumber(%q)", r)
	}

	// Not numerals: ASCII digits (Nd), plain letters, and CJK ideographic numbers (Lo) are excluded.
	for _, r := range []rune{'5', 'A', '一' /* CJK one, category Lo */} {
		_, ok := RuneNumber(r)
		assert.Falsef(t, ok, "RuneNumber(%q) should not be a numeral", r)
	}
}

func TestNumberWordsNumeralRunes(t *testing.T) {
	t.Parallel()

	m := MakeNumberMangler()
	cases := map[string]string{
		"½":         "one half",
		"⅐":         "one seventh", // 1/7 — newly supported base
		"⅚":         "five sixths", // 5/6 — newly supported base
		"⅑":         "one ninth",   // 1/9 — newly supported base
		"⅒":         "one tenth",
		"Ⅶ":         "seven",
		"②":         "two",
		"the ½ cup": "the one half cup",
		"café":      "café", // no numeral, no digit: untouched
	}
	for in, want := range cases {
		assert.EqualTf(t, want, m.NumberWords(in), "NumberWords(%q)", in)
	}
}

// TestFractionBasesExtended verifies 1/6, 1/7, 1/9 are now recognized simple fractions (not just via
// numeral runes but for plain decimal input too, since the base set is global).
func TestFractionBasesExtended(t *testing.T) {
	t.Parallel()

	m := MakeNumberMangler()
	cases := map[string]string{
		"0.16667": "one sixth",
		"0.83333": "five sixths",
		"0.14286": "one seventh",
		"0.11111": "one ninth",
	}
	for in, want := range cases {
		assert.EqualTf(t, want, m.NumberWords(in), "NumberWords(%q)", in)
	}
}
