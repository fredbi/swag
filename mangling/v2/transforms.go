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
