package mangling

import (
	"unicode"
	"unicode/utf8"
)

// defaultTokenSeparator reports whether a rune is a token separator — a rune that is *elided* (dropped, never
// emitted) and marks a boundary between tokens — using the built-in [defaultSymbolWords] set.
//
// It is the default injected into [github.com/go-openapi/swag/mangling/v2/internal/tokens.Tokenizer].Separator when
// the symbol set is not customized, and it implements bucket 3 of the segmentation classification (see also
// [defaultSymbolWords]):
//
//  1. letters & digits    -> token content (never a separator).
//  2. verbalized symbols  -> NOT a separator. A rune in the symbol set (@ ! # & . …) becomes
//     its own single-rune *symbol token*; whether it is then dropped or turned into a word is the
//     target's symbol policy, decided downstream — not here. This is why "." can both be
//     elided for an identifier (Index01) and spelled "dot" when a target verbalizes.
//  3. everything else      -> separator (this function): whitespace, non-printable runes, and the
//     structural punctuation categories not claimed by bucket 2.
//
// Design notes:
//   - !unicode.IsGraphic covers control (Cc), format (Cf: zero-widths, BOM, soft hyphen) and
//     line/paragraph separators (Zl, Zp), plus surrogate/private/unassigned — no explicit test needed.
//   - Symbol categories Sm/Sc/So (+ < = > | ~ $ …) are intentionally NOT tested by category: a symbol
//     we verbalize is pulled out by bucket 2; one we don't falls through as a symbol token (→ drop or
//     rune-name fallback downstream). Plain unicode.IsPunct was both too wide (ate @ ! #) and too
//     narrow (missed + < = >); this split fixes both.
//   - Sk (modifier symbols: backtick, spacing accents ´ ¨ ¯ ¸ ˆ ˜ …) ARE elided, except those the
//     word map claims (e.g. ^ -> caret), which map-first keeps as symbol tokens.
//
// Segmentation touches every rune and most runes are ASCII, so the ASCII decisions for the default set are precomputed
// into [asciiSeparator]: the hot path is a single array lookup, and only non-ASCII runs the full category test. A
// mangler with a *custom* symbol set builds its own predicate at construction (see [separatorRule]), so this shared
// default stays allocation-free.
func defaultTokenSeparator(r rune) bool {
	if r < utf8.RuneSelf {
		return asciiSeparator[r]
	}

	return separatorForRune(r, defaultSymbolWords)
}

// asciiSeparator caches the default-set separator decision for every ASCII rune, filled in init.
var asciiSeparator [utf8.RuneSelf]bool

func init() {
	for r := rune(0); r < utf8.RuneSelf; r++ {
		asciiSeparator[r] = separatorForRune(r, defaultSymbolWords)
	}
}

// separatorForRune is the full separator predicate for a given symbol set; see [defaultTokenSeparator] for the
// rationale. A rune present in words is a verbalized symbol token, not a separator.
func separatorForRune(r rune, words map[rune]string) bool {
	if _, verbalize := words[r]; verbalize {
		return false // bucket 2: a symbol token, not a separator
	}

	return unicode.IsSpace(r) ||
		!unicode.IsGraphic(r) || // control, format, line/para separators, surrogate, private, unassigned
		unicode.In(r,
			unicode.Pd, // dashes / hyphens
			unicode.Ps, // open brackets/parens/braces
			unicode.Pe, // close brackets/parens/braces
			unicode.Pi, // initial quotes
			unicode.Pf, // final quotes
			unicode.Pc, // connectors (underscore, ties)
			unicode.Po, // other punctuation (comma, colon, semicolon, … minus the verbalized ones)
			unicode.Sk, // modifier symbols: backtick, spacing accents (´ ¨ ¯ ¸ ˆ ˜ …), minus verbalized ones (^)
		)
}

// separatorRule is the per-mangler separator predicate for a *customized* symbol set.
//
// It mirrors the shared default: an ASCII lookup table plus the full category test for non-ASCII. The receiver is
// deliberately small (two pointers), so binding its [separatorRule.sep] method value into the tokenizer — the way the
// custom predicate is injected — captures 16 bytes, not the whole (value-receiver) mangler, and the [utf8.RuneSelf]bool
// table lives behind a pointer instead of bloating every mangler method call.
type separatorRule struct {
	ascii *[utf8.RuneSelf]bool
	words map[rune]string
}

// newSeparatorRule precomputes the ASCII table for a custom symbol set (once, at mangler construction).
func newSeparatorRule(words map[rune]string) separatorRule {
	var table [utf8.RuneSelf]bool
	for r := rune(0); r < utf8.RuneSelf; r++ {
		table[r] = separatorForRune(r, words)
	}

	return separatorRule{ascii: &table, words: words}
}

// sep is the injected predicate: an ASCII array lookup, or the full category test for non-ASCII.
func (s separatorRule) sep(r rune) bool {
	if r < utf8.RuneSelf {
		return s.ascii[r]
	}

	return separatorForRune(r, s.words)
}
