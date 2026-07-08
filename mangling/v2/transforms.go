package mangling

import (
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/swag/mangling/v2/numbers"
	"github.com/go-openapi/swag/mangling/v2/runewords"
	"github.com/go-openapi/swag/pools/shared"
)

// TargetTransform is a compiled, immutable recipe describing how to render a segmented token stream: casing ×
// separator × affix × stages × repair.
//
// All fields are unexported; build custom targets with [MakeTargetTransform].
//
// The mangler supplies the data (dictionaries) that stages bind to at run time, so a target degrades gracefully across
// manglers.
//
// Fields are unexported; the assembly recipe is casing × separator × symbol-policy (affix, stages and repair land
// later).
type TargetTransform struct {
	firstCasing  wordCasing   // casing of the first emitted word (camelCase lowercases it)
	restCasing   wordCasing   // casing of subsequent words
	separator    string       // "", "_", "-", " ", "."
	symbolPolicy symbolPolicy // drop | verbalize | keep
}

// MakeTargetTransform builds a custom [TargetTransform].
func MakeTargetTransform(opts ...TargetOption) TargetTransform {
	var tr TargetTransform
	for _, apply := range opts {
		tr = apply(tr)
	}

	return tr
}

// TargetOption customizes a [TargetTransform].
type TargetOption func(TargetTransform) TargetTransform

// WithSeparator sets the output separator emitted between tokens.
func WithSeparator(sep string) TargetOption {
	return func(tr TargetTransform) TargetTransform {
		tr.separator = sep

		return tr
	}
}

// Preset targets.
//
// These return a fresh immutable value. (they are functions, not variables, so a caller can never corrupt a shared
// preset).
// Named after the form they produce.

// TargetTitle produces space-separated tokens, each capitalized ("Title Case").
func TargetTitle() TargetTransform {
	return TargetTransform{firstCasing: casingTitle, restCasing: casingTitle, separator: " "}
}

// TargetSentence produces space-separated tokens, only the first capitalized ("Sentence case").
func TargetSentence() TargetTransform {
	return TargetTransform{firstCasing: casingTitle, restCasing: casingLower, separator: " "}
}

// TargetSnake produces underscore-separated lower-case tokens ("snake_case").
func TargetSnake() TargetTransform {
	return TargetTransform{firstCasing: casingLower, restCasing: casingLower, separator: "_"}
}

// TargetKebab produces hyphen-separated lower-case tokens ("kebab-case").
func TargetKebab() TargetTransform {
	return TargetTransform{firstCasing: casingLower, restCasing: casingLower, separator: "-"}
}

// TargetCamel produces joined tokens with a lower-case first token and the rest capitalized ("camelCase").
func TargetCamel() TargetTransform {
	return TargetTransform{firstCasing: casingLower, restCasing: casingTitle}
}

// TargetPascal produces joined tokens, each capitalized ("PascalCase").
func TargetPascal() TargetTransform {
	return TargetTransform{firstCasing: casingTitle, restCasing: casingTitle}
}

// TargetAllCaps produces underscore-separated upper-case tokens ("ALL_CAPS").
func TargetAllCaps() TargetTransform {
	return TargetTransform{firstCasing: casingUpper, restCasing: casingUpper, separator: "_"}
}

// expandRuneNames is the rune-name tier of asciification.
//
// Every non-ASCII rune that diacritic folding won't handle (non-Latin letters, symbols, single-codepoint emoji) is
// replaced by its space-delimited phonetic name (π → " pi ", 😀 → " grinning face ") so it re-segments into
// words and re-cases per word (GrinningFace, not "Grinning face").
//
// Runes the table elides (CJK ideographs, decorative symbols) are dropped.
// Foldable diacritics and combining marks pass through untouched for the token-level fold stage.
// Allocates only when a substitution or drop is actually needed.
//
// The scratch buffer comes from a pool so its (heavy) working allocation is amortized across calls — unlike a
// strings.Builder, whose buffer is discarded each call; only the final materialized string allocates per call.
//
// Branch order is chosen for the hot case: [runewords.Word] is tried first for a non-ASCII rune, so a named rune
// (Greek, Cyrillic, emoji, symbol — the common non-ASCII input) resolves in one table lookup, skipping the
// combining-mark / diacritic / numeral probes that a non-Latin letter would otherwise all miss.
func expandRuneNames(str string) string {
	// Find the first rune this pass must act on: non-ASCII and not foldable (a diacritic/combining mark passes through
	// for the fold stage). Everything before it — ASCII and foldable runes — is unchanged, so it is bulk-copied rather
	// than re-scanned; if there is no such rune, the input is returned untouched, with no buffer and no copy.
	start := -1
	for i, r := range str {
		if r >= utf8.RuneSelf && !foldable(r) {
			start = i

			break
		}
	}
	if start < 0 {
		return str
	}

	b, redeem := shared.BorrowBufferWithRedeem()
	defer redeem()
	b.Grow(2 * len(str))       // rough; the pooled buffer's capacity is recycled, so any regrow amortizes across calls
	b.WriteString(str[:start]) // the unchanged prefix, in one copy

	for _, r := range str[start:] {
		switch {
		case r < utf8.RuneSelf:
			b.WriteRune(r) // ASCII passes through (one byte)
		default:
			if w, ok := runewords.Word(r); ok { // the common non-ASCII case: name the rune (Greek, Cyrillic, emoji, symbol)
				b.WriteByte(' ')
				b.WriteString(w)
				b.WriteByte(' ')
			} else if foldable(r) {
				b.WriteRune(r) // foldable diacritic or combining mark: passed through for the token-level fold stage
			} else if d, ok := asciiDigit(r); ok {
				b.WriteByte(d) // non-ASCII decimal digit (Nd) → its ASCII digit ('٧' → '7')
			} else if words := numbers.NumberRune(r); words != "" { // numeral rune → words ("½" → "one half")
				b.WriteByte(' ')
				b.WriteString(words)
				b.WriteByte(' ')
			} // else: an elided rune (CJK ideograph, decorative symbol) — dropped
		}
	}

	return b.String() // the one per-call allocation (a copy); the pooled buffer's capacity is recycled on redeem
}

// operatorWords verbalizes an operator sequence as a multi-word phrase, expanded before segmentation (like
// [expandRuneNames]) so the phrase re-segments and cases per word: "!=" → " not equal " → [not, equal] → "NotEqual".
//
// A single cased symbol word cannot do this: the assembler cases a symbol replacement as one word, so "not equal"
// would come out "Not equal". The Unicode glyphs mirror their ASCII twins (≠ = !=); pinning them here also fixes the
// generic rune-name collapse, which drops the load-bearing word (≠'s name "NOT EQUAL TO" reduces to "equal").
//
// Keys are at most two runes. Values are uninflected base forms — "equal"/"match"/"imply", not the 3rd-person
// "equals"/"matches"/"implies" — so they read as neutral labels and stay consistent with the per-rune symbol table
// (both "=" and "==" verbalize to "equal").
var operatorWords = map[string]string{
	// ASCII digraphs
	"!=": "not equal", "==": "equal",
	"<=": "less or equal", ">=": "greater or equal",
	"&&": "and", "||": "or",
	"<<": "shift left", ">>": "shift right",
	"**": "power", "::": "scope",
	"->": "to", "=>": "imply",
	"++": "increment", "--": "decrement",
	"=~": "match", "!~": "not match",
	// single-char comparisons (multi-word, so owned here rather than the per-rune symbol table)
	"<": "less than", ">": "greater than",
	// Unicode operator glyphs, pinned to their ASCII twins
	"≠": "not equal", "≤": "less or equal", "≥": "greater or equal",
	"→": "to", "⇒": "imply", "≈": "approximately", "≡": "equivalent", "¬": "not",
}

// operatorLeadByte[c] reports whether byte c can start an [operatorWords] key — the first byte of each key (an ASCII
// key contributes its byte; a glyph key its UTF-8 lead byte, 0xE2 for the U+2xxx glyphs or 0xC2 for ¬). It lets the
// scan skip an ordinary character — including any non-glyph non-ASCII rune such as CJK — with a single array lookup
// and no map lookup.
//
// Derived from [operatorWords] in init, so it never drifts.
var operatorLeadByte [256]bool

func init() {
	for k := range operatorWords {
		operatorLeadByte[k[0]] = true
	}
}

// expandOperators replaces operator sequences with their space-padded words, ahead of segmentation, so a multi-word
// operator re-segments and cases per word.
//
// It is not gated on ASCII folding — verbalizing "!=" is a symbol concern, not a folding one. (A target whose symbol
// policy is *drop* is not honored here, since drop is an assembly-time decision and this runs pre-segmentation.)
//
// It works directly on the string (no []rune) and only allocates once an operator is actually substituted: an input
// with a lead byte but no operator (e.g. "a-b") passes through untouched and allocation-free. A map lookup happens only
// at a lead byte (or any non-ASCII byte, which might begin a glyph); ordinary characters cost a single array lookup.
func expandOperators(str string) string {
	const expansionMargin = 16

	var b strings.Builder

	last := 0
	for i := 0; i < len(str); {
		c := str[i]

		if operatorLeadByte[c] {
			if w, size := operatorAt(str, i); size > 0 {
				if last == 0 {
					b.Grow(len(str) + expansionMargin)
				}

				b.WriteString(str[last:i])
				b.WriteByte(' ')
				b.WriteString(w)
				b.WriteByte(' ')
				i += size
				last = i

				continue
			}
		}

		// ordinary byte, or a lead byte with no operator (e.g. a lone '-'): advance one rune, ASCII inline.
		if c < utf8.RuneSelf {
			i++
		} else {
			_, size := utf8.DecodeRuneInString(str[i:])
			i += size
		}
	}

	if last == 0 {
		return str // no operator substituted: original string, no allocation
	}

	b.WriteString(str[last:])

	return b.String()
}

// operatorAt returns the operator word beginning at byte i and its byte length, greedy longest-first (two-rune key
// before one-rune), or ("", 0) if none. Byte substrings are used as map keys, so it does not allocate.
func operatorAt(str string, i int) (string, int) {
	_, s1 := utf8.DecodeRuneInString(str[i:])

	if i+s1 < len(str) {
		_, s2 := utf8.DecodeRuneInString(str[i+s1:])
		if w, ok := operatorWords[str[i:i+s1+s2]]; ok {
			return w, s1 + s2
		}
	}

	if w, ok := operatorWords[str[i:i+s1]]; ok {
		return w, s1
	}

	return "", 0
}
