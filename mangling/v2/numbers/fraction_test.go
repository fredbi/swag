// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

func TestFraction(t *testing.T) {
	t.Parallel()

	for tc := range fractionCases() {
		out, ok := fraction(tc.x, tc.o)
		assert.EqualTf(t, tc.isOK, ok, "fraction(%v) ok", tc.x)
		assert.EqualTf(t, tc.out, out, "fraction(%v)", tc.x)
	}
}

func TestSpellDecimal(t *testing.T) {
	t.Parallel()

	for tc := range decimalCases() {
		assert.EqualTf(t, tc.out, spellDecimal(tc.s, numberOptions{}), "spellDecimal(%q)", tc.s)
	}
}

func TestNumberWords(t *testing.T) {
	t.Parallel()

	// numberWords is the internal value verbalizer (there is no exported value helper)
	assert.EqualT(t, "twelve", numberWords(12, numberOptions{}))
	assert.EqualT(t, "three hundred", numberWords(300, numberOptions{}))
	assert.EqualT(t, "one quarter", numberWords(0.25, numberOptions{}))
	assert.EqualT(t, "one tenth", numberWords(0.1, numberOptions{}))
	assert.EqualT(t, "one third", numberWords(1.0/3.0, numberOptions{}))
	assert.EqualT(t, "zero", numberWords(0, numberOptions{}))
}

// =============================================
// Fractions
// =============================================

type fractionCase struct {
	x    float64
	o    numberOptions
	out  string
	isOK bool
}

func fractionCases() iter.Seq[fractionCase] {
	return slices.Values([]fractionCase{
		{0.5, numberOptions{}, "one half", true},
		{0.25, numberOptions{}, "one quarter", true},
		{0.75, numberOptions{}, "three quarters", true},
		{0.2, numberOptions{}, "one fifth", true},
		{0.4, numberOptions{}, "two fifths", true},
		{0.6, numberOptions{}, "three fifths", true},
		{0.125, numberOptions{}, "one eighth", true},
		{0.375, numberOptions{}, "three eighths", true},
		{0.1, numberOptions{}, "one tenth", true},
		{0.9, numberOptions{}, "nine tenths", true},
		{1.0 / 3.0, numberOptions{}, "one third", true},
		{2.0 / 3.0, numberOptions{}, "two thirds", true},
		{0.333, numberOptions{}, "one third", true}, // within tolerance
		{-0.25, numberOptions{}, "minus one quarter", true},
		{0.1, numberOptions{stripOne: true}, "tenth", true},
		{0.5, numberOptions{stripOne: true}, "half", true},
		{0.31456, numberOptions{}, "", false}, // not a simple fraction
		{0.123, numberOptions{}, "", false},
	})
}

type decimalCase struct {
	s   string
	out string
}

func decimalCases() iter.Seq[decimalCase] {
	return slices.Values([]decimalCase{
		{"123", "one hundred and twenty three"},
		{"0.25", "one quarter"},
		{"0.5", "one half"},
		{"-0.75", "minus three quarters"},
		{"0.31456", "zero dot three one four five six"},
		{"1.25", "one dot two five"},
		{"1.5", "one dot five"},
		{"2.718", "two dot seven one eight"},
		{"-1.5", "minus one dot five"},
		{"0.16667", "one sixth"},
		{"0.83333", "five sixths"},
		{"0.14286", "one seventh"},
		{"0.11111", "one ninth"},
	})
}
