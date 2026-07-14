// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

// NumberMangler verbalizes the numbers found in text as English words: cardinals and common fractions, with
// digit-group (thousands) reconstruction.
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
	if r < 0x00B2 {
		// No No/Nl numeral exists below U+00B2 (superscript two), so ASCII and low-Latin runes skip the map lookup —
		// this is called per rune while scanning text.
		return 0, false
	}

	v, ok := runeNumericValue[r]

	return v, ok
}

// NumberRune verbalizes a single Unicode numeral rune as English words ('½' → "one half", 'Ⅶ' → "seven", '②' →
// "two"), or returns "" when r is not a numeral rune (categories No and Nl; see [RuneNumber]).
//
// It is the single-rune form of [NumberMangler.NumberWords]: it resolves the rune's value with [RuneNumber] and
// renders it directly, skipping the string scanner and the string(r) allocation that NumberWords(string(r)) would
// cost — this runs per numeral rune in the asciify pass.
//
// Rendering honors the mangler's options ([WithNumberStripOne] etc.), so a numeral rune verbalizes consistently with
// the same value written as digits ('½' and "1/2"-style input agree under one mangler). Use the package-level
// [NumberRune] for a quick default-options rendering without a mangler.
func (m NumberMangler) NumberRune(r rune) string {
	v, ok := RuneNumber(r)
	if !ok {
		return ""
	}

	return numberWords(v, m.numberOptions)
}

// NumberRune is the default-options form of [NumberMangler.NumberRune]: it verbalizes a single Unicode numeral rune
// with no options applied. Build a [NumberMangler] and call its method when options matter.
func NumberRune(r rune) string {
	var m NumberMangler // zero value: default options

	return m.NumberRune(r)
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

// The engine is reached through NumberMangler (which verbalizes numbers found in text), RuneNumber (which resolves a
// numeral rune to its value), NumberRune (which verbalizes a single numeral rune — as a package function with default
// options, or as a NumberMangler method that honors the mangler's options), and Roman (which renders an integer as a
// roman numeral).
// The internal numberWords helper verbalizes a single cardinal value; a typed public cardinal helper can be added later
// if a consumer needs one, without breaking callers.
