// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import (
	"math"
	"strconv"
	"strings"
)

// fractionBases are the recognized simple fractions, in descending value so the largest base (the simplest fraction) is
// matched first:
//
// 0.5 → "one half" rather than "two quarters".
var fractionBases = []struct {
	value            float64
	singular, plural string
}{
	{0.5, "half", "halves"},
	{1.0 / 3.0, "third", "thirds"},
	{0.25, "quarter", "quarters"},
	{0.2, "fifth", "fifths"},
	{1.0 / 6.0, "sixth", "sixths"},
	{1.0 / 7.0, "seventh", "sevenths"},
	{0.125, "eighth", "eighths"},
	{1.0 / 9.0, "ninth", "ninths"},
	{0.1, "tenth", "tenths"},
}

// tolerance is the absolute error allowed in fraction detection: 10^-precision (default precision 3).
func (o numberOptions) tolerance() float64 {
	const powBase = 10

	p := o.precision
	if p == 0 {
		p = 3
	}

	return math.Pow(powBase, -float64(p))
}

// fraction recognizes x ∈ (-1, 1) as one of the [fractionBases]: if x ≈ n·baseᵢ within tolerance, it renders
// "{cardinal n} {base name}" (pluralized when n > 1).
//
// Returns ("", false) if none matches.
//
//	0.25 → "one quarter"   0.75 → "three quarters"   0.1 → "one tenth" ("tenth" with StripOne)
func fraction(x float64, o numberOptions) (string, bool) {
	var b buf
	ok := writeFraction(&b, x, o)

	return unsafeStr(b.b), ok
}

// writeFraction streams [fraction] into b and reports whether x matched a simple fraction (writing nothing if not).
func writeFraction(b *buf, x float64, o numberOptions) bool {
	sign := false
	if x < 0 {
		sign, x = true, -x
	}

	tol := o.tolerance()
	for _, base := range fractionBases {
		n := int64(math.Round(x / base.value))
		if n < 1 || math.Abs(x-float64(n)*base.value) > tol {
			continue
		}

		name := base.singular
		if n > 1 {
			name = base.plural
		}
		if sign {
			_, _ = b.WriteString("minus ")
		}
		if n == 1 && o.stripOne {
			_, _ = b.WriteString(name) // "tenth"

			return true
		}

		writeCardinal(b, n, o)
		_ = b.WriteByte(' ')
		_, _ = b.WriteString(name) // "one tenth" / "three quarters"

		return true
	}

	return false
}

// spellDecimal renders a numeric string as words.
//
// A value in (-1, 1) that matches a simple fraction is spelled as that fraction; otherwise a decimal is spelled
// digit-by-digit after "dot" ("0.31456" → "zero dot three one four five six").
//
// Integers go straight to [cardinal].
func spellDecimal(s string, o numberOptions) string {
	var b buf
	writeSpellDecimal(&b, s, o)

	return unsafeStr(b.b)
}

// writeSpellDecimal streams [spellDecimal] into b.
func writeSpellDecimal(b *buf, s string, o numberOptions) {
	if name, ok := matchSpecial(s, o); ok {
		_, _ = b.WriteString(name)

		return
	}

	if !strings.ContainsRune(s, '.') {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			writeCardinal(b, n, o)

			return
		}

		// Integer too large for int64: spell it out one word per digit so it is still fully verbalized — never left as raw
		// digits, which would make an identifier start with a digit.
		body := s
		if rest, ok := strings.CutPrefix(body, "-"); ok {
			_, _ = b.WriteString("minus ")
			body = rest
		}
		body = strings.TrimPrefix(body, "+")
		writeDigitWords(b, body)

		return
	}

	x, err := strconv.ParseFloat(s, 64)
	if err != nil {
		_, _ = b.WriteString(s)

		return
	}
	if x > -1 && x < 1 && x != 0 {
		if writeFraction(b, x, o) {
			return
		}
	}

	body := s
	if rest, ok := strings.CutPrefix(body, "-"); ok {
		_, _ = b.WriteString("minus ")
		body = rest
	}
	body = strings.TrimPrefix(body, "+")

	intPart, fracPart, _ := strings.Cut(body, ".")
	if intPart == "" {
		intPart = "0"
	}
	intVal, _ := strconv.ParseInt(intPart, 10, 64)

	writeCardinal(b, intVal, o)
	_, _ = b.WriteString(" dot ")
	writeDigitWords(b, fracPart)
}

// numberWords renders a numeric value as words: cardinal for integral values, fraction/decimal otherwise (the value is
// formatted to its shortest exact decimal string first).
func numberWords(x float64, o numberOptions) string {
	if x == math.Trunc(x) {
		return cardinal(int64(x), o)
	}

	return spellDecimal(strconv.FormatFloat(x, 'f', -1, 64), o)
}

// writeNumberValue streams the english wording of a numeric value into b — the streaming form of [numberWords], used
// to verbalize a Unicode numeral rune ('½' → "one half", 'Ⅶ' → "seven").
func writeNumberValue(b *buf, x float64, o numberOptions) {
	if x == math.Trunc(x) {
		writeCardinal(b, int64(x), o)

		return
	}

	writeSpellDecimal(b, strconv.FormatFloat(x, 'f', -1, 64), o)
}

// writeDigitWords spells a run of digits one by one into b: "31456" → "three one four five six".
func writeDigitWords(b *buf, digits string) {
	first := true
	for _, r := range digits {
		if r >= '0' && r <= '9' {
			if !first {
				_ = b.WriteByte(' ')
			}
			_, _ = b.WriteString(cardinalOnes[r-'0'])
			first = false
		}
	}
}
