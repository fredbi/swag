// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

func TestRoman(t *testing.T) {
	t.Parallel()

	for tc := range romanCases() {
		assert.EqualTf(t, tc.out, Roman(int64(tc.n)), "Roman(%d)", tc.n)
	}
}

type romanCase struct {
	n   int
	out string
}

func romanCases() iter.Seq[romanCase] {
	return slices.Values([]romanCase{
		{0, ""},
		{-5, ""},
		{1, "i"},
		{4, "iv"},
		{6, "vi"},
		{9, "ix"},
		{12, "xii"},
		{40, "xl"},
		{90, "xc"},
		{400, "cd"},
		{900, "cm"},
		{1994, "mcmxciv"},
		{2024, "mmxxiv"},
		{3999, "mmmcmxcix"},
		{4000, "mmmm"}, // no cap: "m" repeats
	})
}
