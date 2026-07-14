// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

func TestCardinal(t *testing.T) {
	t.Parallel()

	for tc := range cardinalCases() {
		assert.EqualTf(t, tc.out, cardinal(tc.n, tc.o), "cardinal(%d, %+v)", tc.n, tc.o)
	}
}

type numberCase struct {
	n   int64
	o   numberOptions
	out string
}

func cardinalCases() iter.Seq[numberCase] {
	return slices.Values([]numberCase{
		{0, numberOptions{}, "zero"},
		{1, numberOptions{}, "one"},
		{9, numberOptions{}, "nine"},
		{10, numberOptions{}, "ten"},
		{15, numberOptions{}, "fifteen"},
		{20, numberOptions{}, "twenty"},
		{23, numberOptions{}, "twenty three"},
		{99, numberOptions{}, "ninety nine"},
		{100, numberOptions{}, "one hundred"},
		{101, numberOptions{}, "one hundred and one"},
		{123, numberOptions{}, "one hundred and twenty three"},
		{999, numberOptions{}, "nine hundred and ninety nine"},
		{1000, numberOptions{}, "one thousand"},
		{1005, numberOptions{}, "one thousand and five"},
		{1234, numberOptions{}, "one thousand two hundred and thirty four"},
		{999999, numberOptions{}, "nine hundred and ninety nine thousand nine hundred and ninety nine"},
		{1000000, numberOptions{}, "one million"},
		{-5, numberOptions{}, "minus five"},
		{-123, numberOptions{}, "minus one hundred and twenty three"},

		// StripOne: drop a leading "one" before a scale
		{100, numberOptions{stripOne: true}, "hundred"},
		{1000, numberOptions{stripOne: true}, "thousand"},
		{1000000, numberOptions{stripOne: true}, "million"},
		{123, numberOptions{stripOne: true}, "hundred and twenty three"},
		{21, numberOptions{stripOne: true}, "twenty one"}, // "one" not before a scale — kept

		// StripAnd: drop the "and"
		{123, numberOptions{stripAnd: true}, "one hundred twenty three"},
		{1005, numberOptions{stripAnd: true}, "one thousand five"},

		// both
		{123, numberOptions{stripOne: true, stripAnd: true}, "hundred twenty three"},

		// hybrid (> 1,000,000) — format pending confirmation
		{1234567, numberOptions{}, "one million 234 thousands and 567"},
	})
}
