package numbers

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

func TestRuneNumber(t *testing.T) {
	t.Parallel()

	const epsilon = 1e-6
	for tc := range numeralFloatCases() {
		v, ok := RuneNumber(tc.r)
		assert.Truef(t, ok, "RuneNumber(%q) should be a numeral", tc.r)
		assert.InDeltaf(t, tc.expected, v, epsilon, "RuneNumber(%q)", tc.r)
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
	for tc := range numeralCases() {
		assert.EqualTf(t, tc.expected, m.NumberWords(tc.in), "NumberWords(%q)", tc.in)
	}
}

type numeralFloatCase struct {
	r        rune
	expected float64
}

func numeralFloatCases() iter.Seq[numeralFloatCase] {
	return slices.Values([]numeralFloatCase{
		{'½', 0.5},
		{'¼', 0.25},
		{'⅐', 1.0 / 7.0},
		{'⅒', 0.1},
		{'²', 2},
		{'Ⅶ', 7}, // roman numeral (Nl)
		{'②', 2}, // circled digit (No)
	})
}

type numeralWordCase struct {
	in       string
	expected string
}

func numeralCases() iter.Seq[numeralWordCase] {
	return slices.Values([]numeralWordCase{
		{
			in:       "½",
			expected: "one half",
		},
		{
			in:       "⅐",
			expected: "one seventh",
		},
		{
			in:       "⅚",
			expected: "five sixths",
		},
		{
			in:       "⅑",
			expected: "one ninth",
		},
		{
			in:       "⅒",
			expected: "one tenth",
		},
		{
			in:       "Ⅶ",
			expected: "seven",
		},
		{
			in:       "②",
			expected: "two",
		},
		{
			in:       "the ½ cup",
			expected: "the one half cup",
		},
		{
			in:       "café",
			expected: "café", // no numeral, no digit: untouched
		},
	})
}
