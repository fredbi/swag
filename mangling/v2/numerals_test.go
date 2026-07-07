package mangling

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// TestNumeralAsciify covers the three distinct treatments of a Unicode numeral rune:
//   - the numbers engine spells it        (ConstName: "½" → "OneHalf")
//   - the asciify tier renders it plainly (ToAscii:   "½" → "0.5")
//   - UnicodeName elides it               (numerals are not phonetic names)
func TestNumeralAsciify(t *testing.T) {
	t.Parallel()

	t.Run("ConstName spells the value", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler()
		cases := map[string]string{
			"½":      "OneHalf",
			"⅐":      "OneSeventh",
			"Ⅶ":      "Seven",
			"area ½": "AreaOneHalf",
		}
		for in, want := range cases {
			assert.EqualTf(t, want, g.ConstName(in), "ConstName(%q)", in)
		}
	})

	t.Run("ToAscii renders a plain number (3-decimal cap)", func(t *testing.T) {
		t.Parallel()
		cases := map[string]string{
			"½":         "0.5",
			"⅐":         "0.143", // 1/7 capped at 3 decimals
			"Ⅶ":         "7",
			"²":         "2",
			"the ½ cup": "the 0.5 cup",
		}
		for in, want := range cases {
			assert.EqualTf(t, want, ToAscii(in), "ToAscii(%q)", in)
		}
	})

	t.Run("UnicodeName elides numerals", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", UnicodeName('½'))
		assert.Equal(t, "", UnicodeName('Ⅶ'))
	})

	// A numeral rune must asciify exactly like its plain-number ASCII form in the general path.
	t.Run("numeral is consistent with its ASCII plain form", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler()
		for _, p := range [][2]string{{"½ cup", "0.5 cup"}, {"²", "2"}} {
			assert.Equalf(t, g.Camelize(p[1]), g.Camelize(p[0]),
				"Camelize(%q) should equal Camelize(%q)", p[0], p[1])
		}
	})
}
