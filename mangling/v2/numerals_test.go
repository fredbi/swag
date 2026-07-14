// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/swag/mangling/v2/numbers"
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

	// A numeral rune reached through the asciify pass (expandRuneNames) must verbalize with the SAME number options as
	// the value written as digits (reached through ConstName/NumberWords) — otherwise "½" would spell differently
	// depending on which path it took.
	t.Run("numeral rune honors number options, consistently with ConstName", func(t *testing.T) {
		t.Parallel()

		g := MakeGoMangler(WithGoNumberOptions(numbers.WithNumberStripOne(true)))
		assert.EqualT(t, "Half", g.ConstName("½"))           // value → NumberWords, "one" stripped
		assert.EqualT(t, "HalfCup", g.IdentExported("½cup")) // rune → expandRuneNames, same option applied
	})
}

// TestVerbalizeLeadingSign covers a leading sign bound to a number: '-' is a separator elsewhere, but directly in front
// of a digit it is the number's sign, so "-1" verbalizes to "MinusOne" (not the sign-stripped "One" it used to give on
// the Ident path). IdentExported and ConstName must agree on the signed forms.
func TestVerbalizeLeadingSign(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler()
	for tc := range leadingSignCases() {
		t.Run(tc.in, func(t *testing.T) {
			assert.EqualTf(t, tc.want, g.IdentExported(tc.in), "IdentExported(%q)", tc.in)
			assert.EqualTf(t, tc.want, g.ConstName(tc.in), "ConstName(%q) should agree", tc.in)
		})
	}
}

func leadingSignCases() iter.Seq[numeralAsciifyCase] {
	return slices.Values([]numeralAsciifyCase{
		{"-1", "MinusOne"},
		{"-5", "MinusFive"},
		{"-3.14", "MinusThreeDotOneFour"},
		{"+1", "One"}, // positive sign is dropped (positive is the default)
		{"+2", "Two"},
	})
}

// TestLeadingSignNotBound checks the sign only binds at the very front, right before a digit: an interior '-' still
// splits, a '-' not followed by a digit stays a separator, and a sign detached from the digit does not bind.
func TestLeadingSignNotBound(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler()
	assert.EqualT(t, "MyName", g.IdentExported("my-name")) // interior '-' splits
	assert.EqualT(t, "Abc", g.IdentExported("-abc"))       // '-' not before a digit → separator
	assert.EqualT(t, "One", g.IdentExported("- 1"))        // sign detached from the digit → not bound
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
