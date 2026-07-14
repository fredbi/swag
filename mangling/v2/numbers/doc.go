// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:generate go run ../ucd/cmd/gen_numerals numbers numerals 15.0.0
//go:generate go run ../ucd/cmd/gen_numerals numbers numerals 17.0.0

// Package numbers verbalizes the numbers found in text as English words.
//
// It is a standalone engine: unlike the name-oriented manglers it does its own number-aware scanning (it must
// see the decimal points and digit-group separators that the general tokenizer elides), so it does not depend on
// a separate tokenizer. The parent mangling package reaches it through GoMangler.ConstName.
//
// # Entry points
//
//   - [NumberMangler] rewrites every number in a string ("level 0.25 here" → "level one quarter here"). See
//     [NumberMangler.NumberWords] for the full rules, and [NumberMangler.AppendWords] for the allocation-free
//     variant.
//   - [RuneNumber] resolves a single Unicode numeral rune to its value ('½' → 0.5, 'Ⅶ' → 7).
//   - [Roman] renders an integer as a lowercase roman numeral (6 → "vi"), handy for compact sequence labels.
//
// # How numbers are recognized
//
// Integers become cardinals ("123" → "one hundred and twenty three"); a value in (-1, 1) matching a simple
// fraction becomes that fraction ("0.25" → "one quarter"); any other decimal is spelled digit-by-digit after
// "dot" ("3.14" → "three dot one four"); negatives are prefixed with "minus". Thousands separators (space, comma
// or underscore before exactly three digits) are reconstructed, and integers too large for int64 are spelled
// digit by digit so no input overflows.
//
// Rendering honors the mangler options: [WithNumberStripOne], [WithNumberStripAnd], [WithNumberDetectPrecision]
// and [WithSpecialNumbers].
package numbers
