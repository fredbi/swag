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

// roman renders a positive integer as a lowercase roman numeral (e.g. 12 → "xii", 1994 → "mcmxciv").
//
// It is undefined for n ≤ 0 (returns "").
//
// There is no upper bound: values above 3999 simply repeat "m" (4000 → "mmmm"), which is unambiguous if
// unconventional (we don't use the vinculum system for large numbers, as it requires non-ASCII characters).
func roman(n int64) string {
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
