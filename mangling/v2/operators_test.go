// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

func TestOperatorVerbalization(t *testing.T) {
	t.Parallel()

	// End-to-end through ConstName: an operator verbalizes as a multi-word phrase and cases per word. The Unicode
	// glyphs mirror their ASCII twins (≠ = !=); "a ≠ b" is the regression that used to collapse to "AEqualB"
	// (inverted) via the generic rune-name path.
	t.Run("ConstName", func(t *testing.T) {
		t.Parallel()

		g := MakeGoMangler()
		for tc := range operatorCases() {
			t.Run(tc.in, func(t *testing.T) {
				assert.EqualTf(t, tc.out, g.ConstName(tc.in), "ConstName(%q)", tc.in)
			})
		}
	})

	// Operators verbalize even for the base Mangler with folding off — verbalizing "!=" is a symbol concern, not a
	// folding one, so the pre-segmentation expansion is not gated on ASCII folding.
	t.Run("base Mangler, folding off", func(t *testing.T) {
		t.Parallel()

		m := MakeMangler()
		assert.EqualT(t, "a_not_equal_b", m.Snakize("a != b"))
		assert.EqualT(t, "a_not_equal_b", m.Snakize("a ≠ b"))
		assert.EqualT(t, "i_increment", m.Snakize("i++"))
	})

	// Lone operators keep their per-rune word; separators and non-operators are untouched (fast path).
	t.Run("no digraph, unchanged", func(t *testing.T) {
		t.Parallel()

		g := MakeGoMangler()
		assert.EqualT(t, "AEqualB", g.ConstName("a = b"))             // single '=' → "equal", consistent with "=="
		assert.EqualT(t, "FiftyPercent", g.ConstName("50%"))          // '%' unaffected
		assert.EqualT(t, "MyName", g.ConstName("my-name"))            // lone '-' is a separator
		assert.EqualT(t, "plain text", expandOperators("plain text")) // fast path: no operator lead char
	})
}

func operatorCases() iter.Seq[inOutCase] {
	return slices.Values([]inOutCase{
		{"a != b", "ANotEqualB"},
		{"a ≠ b", "ANotEqualB"}, // regression: ≠ used to invert to "AEqualB"
		{"a == b", "AEqualB"},
		{"x <= y", "XLessOrEqualY"},
		{"x ≤ y", "XLessOrEqualY"},
		{"p >= q", "PGreaterOrEqualQ"},
		{"p ≥ q", "PGreaterOrEqualQ"},
		{"a < b", "ALessThanB"},
		{"a > b", "AGreaterThanB"},
		{"p && q", "PAndQ"},
		{"p || q", "POrQ"},
		{"i++", "IIncrement"},
		{"i--", "IDecrement"},
		{"a =~ b", "AMatchB"},
		{"a !~ b", "ANotMatchB"},
		{"a -> b", "AToB"},
		{"a → b", "AToB"},
		{"a => b", "AImplyB"},
		{"a ⇒ b", "AImplyB"},
		{"x ** y", "XPowerY"},
		{"a :: b", "AScopeB"},
		{"a << b", "AShiftLeftB"},
		{"a >> b", "AShiftRightB"},
		{"a ≈ b", "AApproximatelyB"},
		{"a ≡ b", "AEquivalentB"},
		{"¬x", "NotX"},
	})
}
