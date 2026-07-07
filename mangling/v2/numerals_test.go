package mangling

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// TestNumeralAsciify covers the three distinct treatments of a Unicode numeral rune:
//
//   - the numbers engine spells it        (ConstName: "½" → "OneHalf")
//   - the asciify tier renders it plainly (ToASCII:   "½" → "0.5")
//   - RuneShortName elides it               (numerals are not phonetic names)
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

	t.Run("ToASCII renders a plain number (3-decimal cap)", func(t *testing.T) {
		t.Parallel()

		cases := map[string]string{
			"½":         "0.5",
			"⅐":         "0.143", // 1/7 capped at 3 decimals
			"Ⅶ":         "7",
			"²":         "2",
			"the ½ cup": "the 0.5 cup",
		}
		for in, want := range cases {
			assert.EqualTf(t, want, ToASCII(in), "ToASCII(%q)", in)
		}
	})

	t.Run("RuneShortName elides numerals", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", RuneShortName('½'))
		assert.Equal(t, "", RuneShortName('Ⅶ'))
	})

	// In the name-mangling paths (Camelize/Ident*), a numeral rune is spelled out as words — a name reads better as
	// OneHalfCup than 0Dot5Cup. (ToASCII keeps the plain number; see the subtest above.)
	t.Run("name manglers spell numeral runes as words", func(t *testing.T) {
		t.Parallel()

		m := MakeMangler(WithASCIIFolding(true))
		assert.EqualT(t, "oneHalfCup", m.Camelize("½ cup"))
		assert.EqualT(t, "Two", m.Pascalize("²"))
		assert.EqualT(t, "AnotherOneHalfPlace", MakeGoMangler().IdentExported("another ½ place"))
	})
}
