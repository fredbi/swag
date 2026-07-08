package mangling

import (
	"iter"
	"slices"
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
		for tc := range constNameNumeralCases() {
			assert.EqualTf(t, tc.want, g.ConstName(tc.in), "ConstName(%q)", tc.in)
		}
	})

	t.Run("ToASCII renders a plain number (3-decimal cap)", func(t *testing.T) {
		t.Parallel()

		for tc := range toASCIINumeralCases() {
			assert.EqualTf(t, tc.want, ToASCII(tc.in), "ToASCII(%q)", tc.in)
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

// numeralAsciifyCase is an input → expected-output case for the numeral asciify treatments.
type numeralAsciifyCase struct{ in, want string }

func constNameNumeralCases() iter.Seq[numeralAsciifyCase] {
	return slices.Values([]numeralAsciifyCase{
		{"½", "OneHalf"},
		{"⅐", "OneSeventh"},
		{"Ⅶ", "Seven"},
		{"area ½", "AreaOneHalf"},
	})
}

func toASCIINumeralCases() iter.Seq[numeralAsciifyCase] {
	return slices.Values([]numeralAsciifyCase{
		{"½", "0.5"},
		{"⅐", "0.143"}, // 1/7 capped at 3 decimals
		{"Ⅶ", "7"},
		{"²", "2"},
		{"the ½ cup", "the 0.5 cup"},
	})
}
