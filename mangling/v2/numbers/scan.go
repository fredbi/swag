// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// scanInto writes in to w, verbalizing each numeric run and copying the rest verbatim.
//
// A numeric run is an optional leading sign (only at a word boundary, so "a-5" keeps the hyphen) followed by digits
// with an optional single interior decimal point.
// Numeric runs are ASCII, so the scan works on bytes — no []rune copy of the input — and each run is a direct
// substring of in (no allocation, and no per-call closure).
//
// Thousands separators are stripped per run before spelling.
//
// A single non-ASCII Unicode numeral rune ('½', 'Ⅶ', '②') verbalizes like an ASCII number; every other rune is
// copied verbatim.
// The non-ASCII branch is reached only after the ASCII fast path, so pure-ASCII input never pays for rune decoding.
func scanInto(w *buf, in string, o numberOptions) {
	for i := 0; i < len(in); {
		if end, ok := numberRunAt(in, i); ok {
			writeSpellDecimal(w, stripThousandsSeps(in[i:end]), o)
			i = end

			continue
		}

		c := in[i]
		if c < utf8.RuneSelf {
			// One byte of ASCII non-number text.
			// A number run only starts on '-'/'+'/digit, so this byte is safe to copy directly.
			_ = w.WriteByte(c)
			i++

			continue
		}

		// Non-ASCII rune: a Unicode numeral verbalizes in place (like an ASCII number run — no padding, so "½"->"one
		// half"); anything else is copied verbatim.
		r, size := utf8.DecodeRuneInString(in[i:])
		if v, ok := RuneNumber(r); ok {
			writeNumberValue(w, v, o)
		} else {
			_, _ = w.WriteString(in[i : i+size])
		}
		i += size
	}
}

// mayHaveNumber reports whether s contains anything the verbalizer would rewrite: an ASCII digit or a Unicode numeral
// rune.
//
// It lets [NumberMangler.NumberWords] / [NumberMangler.AppendWords] return the input untouched (no allocation) for
// plain text — including accented text with no numerals.
// The ASCII bytes are scanned directly; only non-ASCII runes are decoded and looked up.
func mayHaveNumber(s string) bool {
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= '0' && c <= '9' {
				return true
			}
			i++

			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if _, ok := RuneNumber(r); ok {
			return true
		}
		i += size
	}

	return false
}

// numberRunAt returns the byte index just past a numeric run starting at byte i in s, or ok=false.
func numberRunAt(s string, i int) (end int, ok bool) {
	j := i

	if s[j] == '-' || s[j] == '+' {
		atBoundary := j == 0
		if !atBoundary {
			r, _ := utf8.DecodeLastRuneInString(s[:j])
			atBoundary = !isWordRune(r)
		}
		if !atBoundary || j+1 >= len(s) || !isASCIIDigit(s[j+1]) {
			return 0, false
		}
		j++
	}

	if j >= len(s) || !isASCIIDigit(s[j]) {
		return 0, false
	}

	// leading digit group
	digitsStart := j
	for j < len(s) && isASCIIDigit(s[j]) {
		j++
	}

	// thousands groups: a leading group of 1..3 digits may be followed by "<sep>ddd" groups, so "1 234", "1,234" and
	// "1_234" all read as 1234 — but "1 2" (not three digits) does not join.
	const digitsGroup = 3
	if j-digitsStart <= digitsGroup {
		for j < len(s) && isThousandsSep(s[j]) && isThreeDigitGroup(s, j+1) {
			j += 4 // separator + exactly three digits
		}
	}

	// optional decimal part
	if j+1 < len(s) && s[j] == '.' && isASCIIDigit(s[j+1]) {
		j++
		for j < len(s) && isASCIIDigit(s[j]) {
			j++
		}
	}

	return j, true
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

func isThousandsSep(b byte) bool { return b == ' ' || b == ',' || b == '_' }

// isThreeDigitGroup reports whether s[k:] begins with exactly three digits (three digits not immediately followed by a
// fourth) — a valid thousands group.
func isThreeDigitGroup(s string, k int) bool {
	if k+3 > len(s) {
		return false
	}
	for x := k; x < k+3; x++ {
		if !isASCIIDigit(s[x]) {
			return false
		}
	}

	return k+3 == len(s) || !isASCIIDigit(s[k+3])
}

// stripThousandsSeps removes the grouping separators from a validated number run ("1 234" -> "1234").
func stripThousandsSeps(num string) string {
	if !strings.ContainsAny(num, " ,_") {
		return num
	}

	return thousandsStripper.Replace(num)
}

var thousandsStripper = strings.NewReplacer(" ", "", ",", "", "_", "")

// isWordRune reports whether r is a letter or digit.
//
// So a '-'/'+' right after it reads as a hyphen or operator rather than a number's sign.
func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
