// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import "maps"

type (
	// NumberOption customizes the behavior of the [NumberMangler].
	NumberOption func(numberOptions) numberOptions

	numberOptions struct {
		stripOne  bool
		stripAnd  bool
		precision uint
		specials  map[string]string
	}
)

func buildNumberOptions(o numberOptions, opts []NumberOption) numberOptions {
	for _, apply := range opts {
		o = apply(o)
	}

	return o
}

// WithNumberStripOne alters how [NumberMangler.NumberWords] renders numerals: whenever stripped the "one" prefix in
// "one hundred", "one tenth", etc is elided.
func WithNumberStripOne(strip bool) NumberOption {
	return func(o numberOptions) numberOptions {
		o.stripOne = strip
		return o
	}
}

// WithNumberStripAnd alters how [NumberMangler.NumberWords] renders numerals: whenever stripped the "and" in "one
// hundred and ten", etc is elided.
func WithNumberStripAnd(strip bool) NumberOption {
	return func(o numberOptions) numberOptions {
		o.stripAnd = strip
		return o
	}
}

// WithNumberDetectPrecision sets the number of decimal places used when matching a value against known
// fractions and special numbers.
//
// A higher precision distinguishes closer values (e.g. 0.333 from 1/3) at the cost of fewer fuzzy matches.
func WithNumberDetectPrecision(precision uint) NumberOption {
	return func(o numberOptions) numberOptions {
		o.precision = precision
		return o
	}
}

// WithSpecialNumbers registers named numeric constants, matched (numerically, within the detection precision) ahead of
// cardinal/fraction rendering.
//
// Example: WithSpecialNumbers(map[string]string{"3.1415": "pi", "2.718": "e"}) renders any number within tolerance of
// 3.1415 as "pi".
func WithSpecialNumbers(specials map[string]string) NumberOption {
	return func(o numberOptions) numberOptions {
		if o.specials == nil {
			o.specials = make(map[string]string, len(specials))
		}
		maps.Copy(o.specials, specials)

		return o
	}
}
