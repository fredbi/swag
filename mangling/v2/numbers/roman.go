// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import "strings"

// romanNumerals is the subtractive value→symbol table, high to low.
var romanNumerals = []struct {
	value  int64
	symbol string
}{
	{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"},
	{100, "c"}, {90, "xc"}, {50, "l"}, {40, "xl"},
	{10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"},
}

// Roman renders a positive integer as a lowercase roman numeral (e.g. 12 → "xii", 1994 → "mcmxciv").
//
// It uses the standard subtractive notation (4 → "iv", not "iiii"; 9 → "ix", not "viiii"), which makes it a handy
// source of compact, human-readable sequence labels — nested loop indices, list markers, and the like:
// "i", "ii", "iii", "iv", "v", … Uppercase the result with strings.ToUpper if you need "IV".
//
// It returns "" for n ≤ 0.
//
// There is no upper bound: values above 3999 simply repeat "m" (4000 → "mmmm"), which is unambiguous if
// unconventional (we don't use the vinculum system for large numbers, as it requires non-ASCII characters).
func Roman(n int64) string {
	if n <= 0 {
		return ""
	}

	var b strings.Builder
	for _, r := range romanNumerals {
		for n >= r.value {
			b.WriteString(r.symbol)
			n -= r.value
		}
	}

	return b.String()
}
