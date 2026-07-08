package mangling

import (
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/swag/mangling/v2/numbers"
	"github.com/go-openapi/swag/mangling/v2/runewords"
)

// numeralVerbalizer spells a numeral rune to words in the asciify pass (½ → "one half").
//
// A default, immutable NumberMangler is enough — the rune-aware scanner turns string(r) into its value's words.
var numeralVerbalizer = numbers.MakeNumberMangler()

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
func expandRuneNames(str string) string {
	need := false
	for _, r := range str {
		if r >= utf8.RuneSelf && !isCombiningMark(r) {
			if _, ok := asciiFold[r]; !ok {
				need = true

				break
			}
		}
	}
	if !need {
		return str // pure ASCII, or only diacritics/combining marks the fold stage handles
	}

	// runes that expand to a word or number make the result longer than the input; a small margin avoids the first
	// reallocation for the common case of a few substitutions.
	const expansionMargin = 16

	var b strings.Builder
	b.Grow(len(str) + expansionMargin)
	for _, r := range str {
		switch {
		case r < utf8.RuneSelf, isCombiningMark(r):
			b.WriteRune(r) // ASCII, or a combining mark left for the fold stage to strip
		default:
			if _, ok := asciiFold[r]; ok {
				b.WriteRune(r) // foldable diacritic: left for the fold stage
			} else if d, ok := asciiDigit(r); ok {
				b.WriteByte(d) // non-ASCII decimal digit (Nd) → its ASCII digit ('٧' → '7'), then handled as a digit
			} else if _, ok := numbers.RuneNumber(r); ok {
				b.WriteByte(' ')
				b.WriteString(numeralVerbalizer.NumberWords(string(r))) // numeral rune → words ("½" → "one half");
				b.WriteByte(' ')                                        // a name reads better spelled out (ToASCII keeps the plain number)
			} else if w, ok := runewords.Word(r); ok {
				b.WriteByte(' ')
				b.WriteString(w)
				b.WriteByte(' ')
			} // else: an elided rune (CJK, decorative) — dropped
		}
	}

	return b.String()
}

// operatorWords verbalizes an operator sequence as a multi-word phrase, expanded before segmentation (like
// [expandRuneNames]) so the phrase re-segments and cases per word: "!=" → " not equal " → [not, equal] → "NotEqual".
//
// A single cased symbol word cannot do this: the assembler cases a symbol replacement as one word, so "not equal"
// would come out "Not equal". The Unicode glyphs mirror their ASCII twins (≠ = !=); pinning them here also fixes the
// generic rune-name collapse, which drops the load-bearing word (≠'s name "NOT EQUAL TO" reduces to "equal").
//
// Keys are at most two runes.
var operatorWords = map[string]string{
	// ASCII digraphs
	"!=": "not equal", "==": "equal",
	"<=": "less or equal", ">=": "greater or equal",
	"&&": "and", "||": "or",
	"<<": "shift left", ">>": "shift right",
	"**": "power", "::": "scope",
	"->": "to", "=>": "implies",
	"++": "increment", "--": "decrement",
	"=~": "matches",
	// single-char comparisons (multi-word, so owned here rather than the per-rune symbol table)
	"<": "less than", ">": "greater than",
	// Unicode operator glyphs, pinned to their ASCII twins
	"≠": "not equal", "≤": "less or equal", "≥": "greater or equal",
	"→": "to", "⇒": "implies", "≈": "approximately", "≡": "equivalent", "¬": "not",
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
