package mangling

import (
	"unicode"
	"unicode/utf8"
)

// defaultTokenSeparator reports whether a rune is a token separator — a rune that is *elided* (dropped, never
// emitted) and marks a boundary between tokens.
//
// It is the default injected into [github.com/go-openapi/swag/mangling/v2/internal/tokens.Tokenizer].Separator, and it
// implements bucket 3 of the segmentation classification (see also [defaultSymbolWords]):
//
//  1. letters & digits    -> token content (never a separator).
//  2. verbalized symbols  -> NOT a separator. A rune in [defaultSymbolWords] (@ ! # & . …) becomes
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
// Segmentation touches every rune and most runes are ASCII, so the ASCII decisions are precomputed into
// [asciiSeparator]: the hot path is a single array lookup, and only non-ASCII runs the full category test.
func defaultTokenSeparator(r rune) bool {
	if r < utf8.RuneSelf {
		return asciiSeparator[r]
	}

	return separatorForRune(r)
}

// asciiSeparator caches [separatorForRune] for every ASCII rune, filled in init.
var asciiSeparator [utf8.RuneSelf]bool

func init() {
	for r := rune(0); r < utf8.RuneSelf; r++ {
		asciiSeparator[r] = separatorForRune(r)
	}
}

// separatorForRune is the full default separator predicate; see [defaultTokenSeparator] for the rationale.
func separatorForRune(r rune) bool {
	if _, verbalize := defaultSymbolWords[r]; verbalize {
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
