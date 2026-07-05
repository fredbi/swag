package numbers

import "maps"

// NumberMangler produces written numerals (cardinals, ordinals, roman) and digit-group aware
// number reconstruction.
//
// It is a standalone engine: unlike the name-oriented manglers it does its own number-aware
// scanning (it must see decimal points and digit-group separators that the general tokenizer elides),
// so it does not depend on the tokenizer.
type NumberMangler struct {
	numberOptions
}

// MakeNumberMangler returns a value [NumberMangler].
func MakeNumberMangler(opts ...NumberOption) NumberMangler {
	var m NumberMangler
	m.numberOptions = buildNumberOptions(m.numberOptions, opts)

	return m
}

// NewNumberMangler returns a pointer to a [NumberMangler].
func NewNumberMangler(opts ...NumberOption) *NumberMangler {
	m := MakeNumberMangler(opts...)

	return &m
}

// NumberWords produces written english numerals.
//
// "123" becomes: "one hundred and twenty three".
//
// Non-numerals present in the string are kept as-is, e.g. "11 and 12" => "eleven and twelve".
//
// # Thousands separators
//
// Groups of digits separated by a comma, a blank space (not a tab) or an underscore are considered as a single group,
// like so:
//
// - 1 234 or 1,234, or 1_234 => one thousand two hundred and thirty four
// - but 1;234 => one;two hundred and thiry four
//
// # Fractional numbers
//
// Simple fractions are identified:
//
//   - 0.1 => one tenth (or simply "tenth" if the NumbersStringone option is true).
//   - 0.25 => one quarter
//   - 0.125 => one eighth
//   - 0.3333 => one third (use 3 decimals precision to infer the fraction)
//
// General decimals are spelled using "dot", as in:
//
//   - 0.31456 => zero dot three FRED TODO
//
// You may use [WithSpecialNumbers] to register specific numeric strings with a given name. Like so:
//
//	  WithSpecialNumbers(map[string]{
//				"3.1415": "pi",
//	     "2.718": "e",
//	     "0.707": "√2/2",
//		 })
//
// Multiple distinct numbers in the same string are processed independently,
// e.g. "10 11" becomes "ten eleven".
func (m NumberMangler) NumberWords(string) string {
	return ""
}

// DigitWords produces written english numerals for digits only.
//
// "123" becomes: "one two three".
//
// The decimal separator "." is spelled "dot": "1.23" => "one dot two three".
//
// Non digits present in the string are kept as-is.
func (m NumberMangler) DigitWords(string) string {
	return ""
}

func NumberWords[T Numerical](n T) string {
	return ""
}

// NumberOrdinal renders an ordinal, e.g. 31 -> 31st.
func NumberOrdinal[T Numerical](n T) string {
	return ""
}

// NumberRoman renders a roman numeral, e.g. 4 -> iv.
func NumberRoman[T Integer](n T) string {
	return ""
}

type (
	// NumberOption customizes the behavior of the [NumberMangler].
	NumberOption func(numberOptions) numberOptions

	numberOptions struct {
		stripOne  bool
		stripAnd  bool
		digits    uint
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

// WithNumberStripOne alters how [NumberMangler.NumberWords] renders numerals:
// whenever stripped the "one" prefix in "one hundred", "one tenth", etc is elided.
func WithNumberStripOne(strip bool) NumberOption {
	return func(o numberOptions) numberOptions {
		o.stripOne = strip
		return o
	}
}

// WithNumberStripAnd alters how [NumberMangler.NumberWords] renders numerals:
// whenever stripped the "and" in "one hundred and ten", etc is elided.
func WithNumberStripAnd(strip bool) NumberOption {
	return func(o numberOptions) numberOptions {
		o.stripAnd = strip
		return o
	}
}

func WithNumberDetectPrecision(precision uint) NumberOption {
	return func(o numberOptions) numberOptions {
		o.precision = precision
		return o
	}
}

// WithSpecialNumbers allow to recognize special numbers.
//
// Example: map[float64]string{"0.314": "pi"} will transform all recognized 0.314 numbers
// (up to the configured detect precision) as "pi" words.
func WithSpecialNumbers(specials map[string]string) NumberOption {
	return func(o numberOptions) numberOptions {
		maps.Copy(o.specials, specials)
		return o
	}
}

type (
	// these type constraints are redefined after golang.org/x/exp/constraints,
	// because importing that package causes an undesired go upgrade.

	// Signed integer types, cf. [golang.org/x/exp/constraints.Signed]
	Signed interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64
	}

	// Unsigned integer types, cf. [golang.org/x/exp/constraints.Unsigned]
	Unsigned interface {
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
	}

	Integer interface {
		Signed | Unsigned
	}
	// Float numerical types, cf. [golang.org/x/exp/constraints.Float]
	Float interface {
		~float32 | ~float64
	}

	// Numerical types
	Numerical interface {
		Signed | Unsigned | Float
	}
)
