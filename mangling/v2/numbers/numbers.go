package numbers

// NumberMangler produces written numerals (cardinals, ordinals, roman) and digit-group aware number reconstruction.
//
// It is a standalone engine: unlike the name-oriented manglers it does its own number-aware scanning (it must see
// decimal points and digit-group separators that the general tokenizer elides), so it does not depend on a separate
// tokenizer.
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
// It leaves the surrounding text untouched, e.g. "10 11" => "ten eleven", "level 0.25 here" => "level one quarter
// here".
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
// digits joins the group, so "1 234", "1,234" and "1_234" all become "one thousand two hundred and thirty four", while
// "1 2" stays two numbers ("one two") and "1;234" is not joined (";" is not a separator).
//
// Registered special numbers ([WithSpecialNumbers]) are matched (within tolerance) ahead of everything else, so
// "3.1415" => "pi".
//
// Rendering honors the mangler's options: [WithNumberStripOne] ("one hundred" => "hundred", "one tenth" => "tenth"),
// [WithNumberStripAnd] (drops the "and"), and [WithNumberDetectPrecision] (fraction and special-number tolerance).
func (m NumberMangler) NumberWords(in string) string {
	if !mayHaveNumber(in) {
		return in // no ASCII digit and no numeral rune: nothing to rewrite, no allocation
	}

	var w buf
	const sensibleGrowth = 16
	w.Grow(len(in) + sensibleGrowth) // verbalized numbers expand (e.g. "200" -> "two hundred")
	scanInto(&w, in, m.numberOptions)

	return unsafeStr(w.b)
}

// RuneNumber returns the numeric value of a Unicode numeral rune (categories No and Nl — e.g. '½' → 0.5, 'Ⅶ' →
// 7, '②' → 2) and whether r is such a numeral.
//
// Decimal digits (Nd) and CJK ideographic numbers (Lo) are deliberately excluded.
// It lets a numeral rune verbalize through this engine ('½' → "one half") and lets the asciify tier render it as a
// plain number ('½' → "0.5").
// Table in numerals.go.
func RuneNumber(r rune) (float64, bool) {
	v, ok := runeNumericValue[r]

	return v, ok
}

// AppendWords appends the english-words form of in (numbers verbalized, surrounding text verbatim) to dst and returns
// the extended slice.
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
	if !mayHaveNumber(in) {
		return append(dst, in...)
	}

	w := buf{b: dst}
	scanInto(&w, in, m.numberOptions)

	return w.b
}

// The engine is reached through NumberMangler (which verbalizes numbers found in text) and RuneNumber (which resolves a
// numeral rune to its value).
// The internal numberWords/roman helpers verbalize a single value; typed public value helpers can be added later if a
// consumer needs them, without breaking callers.
