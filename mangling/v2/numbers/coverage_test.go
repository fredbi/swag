// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// TestNumberWordsOverflow covers the writeSpellDecimal fallback: an integer too large for int64 is spelled digit by
// digit (so it is still verbalized and never leaves a leading digit).
func TestNumberWordsOverflow(t *testing.T) {
	t.Parallel()

	m := MakeNumberMangler()

	got := m.NumberWords("10000000000000000000") // 10^19, > int64 max
	assert.EqualT(t, "one"+strings.Repeat(" zero", 19), got)

	neg := m.NumberWords("-10000000000000000000")
	assert.Falsef(t, strings.ContainsAny(neg, "0123456789"), "overflow number left raw digits: %q", neg)
	assert.EqualT(t, "minus one"+strings.Repeat(" zero", 19), neg)
}

// TestWithNumberDetectPrecision covers the tolerance knob: a tighter precision stops a loose decimal from matching a
// simple fraction.
func TestWithNumberDetectPrecision(t *testing.T) {
	t.Parallel()

	assert.EqualT(t, "one third", MakeNumberMangler().NumberWords("0.333")) // default precision 3: matches 1/3
	//nolint:dupword // "three three three" is the correct digit-by-digit spelling of 0.333
	assert.EqualT(t, "zero dot three three three",
		MakeNumberMangler(WithNumberDetectPrecision(6)).NumberWords("0.333")) // precision 6: no longer 1/3
}

func TestNewNumberMangler(t *testing.T) {
	t.Parallel()

	require := NewNumberMangler()
	assert.NotNil(t, require)
	assert.EqualT(t, MakeNumberMangler().NumberWords("123"), require.NumberWords("123"))
}
