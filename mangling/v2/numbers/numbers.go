package numbers

// NumberMangler produces written numerals (cardinals, ordinals, roman) and digit-group aware number reconstruction.
//
// It is a standalone engine: unlike the name-oriented manglers it does its own number-aware scanning
// (it must see decimal points and digit-group separators that the general tokenizer elides),
// so it does not depend on a separate tokenizer.
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

// NumberWords rewrites every number found in a string as english words.
//
// It leaves the surrounding text untouched,
// e.g. "10 11" => "ten eleven", "level 0.25 here" => "level one quarter here".
//
// Multiple numbers are handled independently.
//
// Each number is an optional sign followed by digits with an optional decimal point:
//
//   - integers become cardinals: "123" => "one hundred and twenty three";
//   - a value in (-1, 1) matching a simple fraction becomes that fraction: "0.25" => "one quarter",
//     "0.1" => "one tenth", "0.75" => "three quarters";
//   - any other decimal is spelled digit-by-digit after "dot": "0.31456" => "zero dot three one four
//     five six";
//   - negatives are prefixed with "minus".
//
// Thousands separators are reconstructed: within a number, a space, comma or underscore followed by exactly three
// digits joins the group, so "1 234", "1,234" and "1_234" all become "one thousand two hundred and thirty four",
// while "1 2" stays two numbers ("one two") and "1;234" is not joined (";" is not a separator).
//
// Registered special numbers ([WithSpecialNumbers]) are matched (within tolerance) ahead of everything else, so
// "3.1415" => "pi".
//
// Rendering honors the mangler's options: [WithNumberStripOne] ("one hundred" => "hundred", "one tenth" => "tenth"),
// [WithNumberStripAnd] (drops the "and"), and [WithNumberDetectPrecision] (fraction and special-number tolerance).
func (m NumberMangler) NumberWords(in string) string {
	if !hasDigit(in) {
		return in // no numeric run possible: nothing to rewrite, no allocation
	}

	var w buf
	const sensibleGrowth = 16
	w.Grow(len(in) + sensibleGrowth) // verbalized numbers expand (e.g. "200" -> "two hundred")
	scanInto(&w, in, m.numberOptions)

	return unsafeStr(w.b)
}

// AppendWords appends the english-words form of in (numbers verbalized, surrounding text verbatim) to
// dst and returns the extended slice.
//
// This is the string-free sibling of [NumberMangler.NumberWords].
//
// The caller owns dst and may reuse it across calls, so bulk verbalization runs allocation-free, like so:
//
//	var scratch []byte
//	for _, s := range inputs {
//		scratch = m.AppendWords(scratch[:0], s)
//		use(scratch) // valid until the next AppendWords into scratch
//	}
func (m NumberMangler) AppendWords(dst []byte, in string) []byte {
	if !hasDigit(in) {
		return append(dst, in...)
	}

	w := buf{b: dst}
	scanInto(&w, in, m.numberOptions)

	return w.b
}

// NumberWords renders a number as english words with default options.
//
// Cardinals for integers ("123" → "one hundred and twenty three"),
// simple fractions or spelled decimals for floats
// ("0.25" → "one quarter", "0.31456" → "zero dot three one four five six").
func NumberWords[T Numerical](n T) string {
	return numberWords(float64(n), numberOptions{})
}

// NumberRoman renders a lowercase roman numeral,
// e.g. 4 -> iv, 12 -> xii.
//
// Undefined (empty) for n <= 0.
func NumberRoman[T Integer](n T) string {
	return roman(int64(n))
}

type (
	// these type constraints are redefined after golang.org/x/exp/constraints

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
