// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package mangling turns arbitrary strings into identifiers and applies well-known recasing operations
// (camelCase, snake_case, kebab-case, ...).
//
// # Manglers
//
// Three manglers build on one pipeline:
//
//   - [Mangler] — general-purpose recasing, ruleset-neutral;
//   - [GoMangler] — identifiers a Go code generator can safely emit (ASCII folding on by default);
//   - [numbers.NumberMangler] — verbalize numbers found in text (subpackage).
//
// Every mangler is an immutable value built with functional options ([MakeMangler] / [MakeGoMangler] return
// a value, [NewMangler] / [NewGoMangler] a pointer) and is safe for concurrent use.
//
// The sections below describe the transforms shared by [Mangler] and [GoMangler]; both type docs refer here
// rather than repeat them.
//
// # Case handling
//
// Casing follows unicode rules: the first letter of a capitalized word is title-cased (via [unicode.ToTitle],
// which differs from uppercase for a handful of digraphs), while ALL-CAPS uses uppercase and the remainder is
// lower-cased.
//
// Special casing rules (e.g. [unicode.SpecialCase]) are not supported at this moment. Letters in languages that
// do not define case remain unchanged.
//
// # Symbol verbalization
//
// Common symbols such as "?", "@", "#" are verbalized and replaced by a short word (e.g. "question", "at",
// "hash"). Multi-rune operators verbalize as a phrase — "!=" → "not equal" → NotEqual, "<=" → LessOrEqual,
// "->" → To — as do their Unicode glyph twins (≠, ≤, →). Operator verbalization runs regardless of ASCII folding;
// it is not honored by a target whose symbol policy drops symbols.
//
// The single-rune set is customizable with [WithSymbolWords] (start from [DefaultSymbolWords]). It is dual-purpose:
// its keys decide which runes are symbol tokens rather than separators, and its values are the emitted words — so
// adding "," makes it a verbalizable token while removing "@" turns it into a separator.
//
// # ASCII folding
//
// By default, letters and digits are left unchanged. With ASCII folding enabled (off in [Mangler], on in
// [GoMangler]), non-ASCII input is reduced to ASCII: Latin letters with diacritics (é, ü) and non-ASCII digits
// fold to their base letter/value, while other non-Latin runes are "phonetized" using their Unicode rune name.
//
// See [ToASCII] for the standalone, tokenization-free folder.
//
// # Numerals
//
// Numbers embedded in values are verbalized through the [numbers] subpackage — cardinals, common fractions and
// digit-group reconstruction. See [numbers.NumberMangler] for the full description of how numbers are recognized.
//
// Within a mangler:
//
//   - ASCII digits are left as-is;
//   - a "." (dot) is verbalized as "dot" (a symbol), a "," (comma) is elided (a separator);
//   - unicode numerals such as ½ verbalize as words when folding is on ("½ cup" → "one half cup"); with folding
//     off they are dropped, like other non-foldable runes. (The standalone [ToASCII] instead renders a numeral
//     as a plain number: "½" → "0.5".)
//
// # Limitations
//
// CJK ideographs and Korean Hangul (both the composed syllables and the standalone Jamo letters) are elided —
// no simple phonetization is available. Unicode grapheme clusters (flag emoji, ZWJ sequences) are not supported
// at this moment.
package mangling
