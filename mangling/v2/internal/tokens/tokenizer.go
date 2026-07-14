// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package tokens

import (
	"fmt"
	"iter"
	"unicode"
)

// Tokenizer splits an UTF-8 string into tokens along opinionated segmentation rules.
//
// # Token boundaries
//
// A run of runes becomes a token; a boundary falls at any of:
//
//   - one or more consecutive separators (identified by [Tokenizer.Separator]); separators are elided, not emitted,
//     e.g. "a, b" => [a, b];
//   - a symbol rune (non-letter, non-digit, non-separator) which becomes its own single-rune token,
//     e.g. "a@b" => [a, @, b];
//   - a letter↔digit transition, e.g. "oauth2" => [oauth, 2], "v4" => [v, 4];
//   - a case alternance:
//   - lower→Upper, e.g. "fooBar" => [foo, Bar];
//   - an Upper-run→lower, with one-rune lookback, e.g. "HTTPServer" => [HTTP, Server].
//
// Combining marks (Mn/Mc/Me) never start a boundary: they attach to the current token (and are stripped later, in the
// fold stage).
//
// A script change between letters (e.g. Latin↔Cyrillic) is intentionally not a boundary yet: with ASCII folding on,
// non-Latin runes are romanized or elided before this point, so it rarely matters.
type Tokenizer struct {
	// Separator reports whether a rune is a token separator (elided, marks a boundary). The root mangler injects its
	// full default (which also keeps verbalized symbols out of the separator set); a nil Separator falls back to a
	// minimal whitespace/non-graphic rule so a bare Tokenizer is still usable.
	Separator func(rune) bool
}

// Tokenize splits a string into its tokens, materialized as strings.
//
// This is a convenience surface (it allocates a string per token).
// The mangling pipeline works on the zero-copy [Tokens] model directly.
func (tk Tokenizer) Tokenize(in string) iter.Seq[string] {
	return func(yield func(string) bool) {
		t := Borrow(in)
		defer t.Redeem()
		tk.Segment(&t)

		for i := range t.Len() {
			if !yield(t.Text(i)) {
				return
			}
		}
	}
}

// Segment fills t with the tokens of its shared []rune, implementing the boundary rules above.
func (tk Tokenizer) Segment(t *Tokens) {
	sep := tk.Separator
	if sep == nil {
		sep = basicSeparator
	}

	runes := t.runes.Slice()
	n := len(runes)

	runStart := -1 // -1 means "no active run"
	var runKind Kind

	flush := func(end int) {
		if runStart < 0 {
			return
		}
		t.push(runStart, end, runKind)
		runStart = -1
	}

	for i := 0; i < n; i++ {
		r := runes[i]
		class := classify(r, sep)

		switch class {
		case classSeparator:
			flush(i) // elided, not emitted

		case classSymbol:
			flush(i)
			t.push(i, i+1, KindSymbol)

		case classMark:
			if runStart < 0 {
				// orphan/leading mark: keep it in a word run so nothing is silently lost (the fold stage will strip it).
				runStart, runKind = i, KindWord
			}
			// otherwise it attaches to the current run (extends on flush)

		case classDigit:
			switch {
			case runStart < 0:
				runStart, runKind = i, KindNumber
			case runKind == KindWord:
				flush(i) // letter↔digit boundary
				runStart, runKind = i, KindNumber
			}
			// else: extend the number run

		case classLetter:
			switch {
			case runStart < 0:
				runStart, runKind = i, KindWord
			case runKind == KindNumber:
				flush(i) // digit↔letter boundary
				runStart, runKind = i, KindWord
			default:
				// letter continuing a word run: check case alternance
				prev, cur := runeCase(runes[i-1]), runeCase(r)
				switch {
				case prev == caseLower && cur == caseUpper:
					// fooBar => foo | Bar
					flush(i)
					runStart, runKind = i, KindWord
				case prev == caseUpper && cur == caseLower && i-1 > runStart && runeCase(runes[i-2]) == caseUpper:
					// Upper-run(>=2) → lower: HTTPServer => HTTP | Server (boundary before the last upper)
					flush(i - 1)
					runStart, runKind = i-1, KindWord
				default:
				}
				// otherwise: same case / single-upper / caseless → extend the run
			}
		default:
			panic(fmt.Errorf("internal error: invalid classification: %v", class))
		}
	}
	flush(n)
}

// basicSeparator is the minimal fallback separator used when [Tokenizer.Separator] is nil: whitespace and any
// non-graphic rune. The root mangler injects a richer default (that also protects verbalized symbols).
func basicSeparator(r rune) bool {
	return unicode.IsSpace(r) || !unicode.IsGraphic(r)
}

// runeClass is a rune's segmentation class.
type runeClass uint8

const (
	classSeparator runeClass = iota // elided boundary
	classLetter                     // word content
	classDigit                      // number content (Nd only)
	classMark                       // combining mark (attaches to the current run)
	classSymbol                     // stand-alone single-rune token
)

func classify(r rune, sep func(rune) bool) runeClass {
	switch {
	case sep(r):
		return classSeparator
	case unicode.IsLetter(r):
		return classLetter
	case unicode.Is(unicode.Nd, r): // decimal digits only; Nl/No (roman, fractions) fall to classSymbol
		return classDigit
	case IsCombiningMark(r): // has its own ASCII/Latin-1 short-circuit
		return classMark
	default:
		return classSymbol
	}
}

// case classes for a rune.
const (
	caseNone = iota
	caseLower
	caseUpper
)

func runeCase(r rune) int {
	switch {
	case unicode.IsUpper(r):
		return caseUpper
	case unicode.IsLower(r):
		return caseLower
	default:
		return caseNone
	}
}

// IsCombiningMark reports whether r is a Unicode combining mark (categories Mn, Mc, Me).
//
// Combining marks never start a token boundary (they attach to the current run) and are never valid identifier
// characters (assembly and the fold stage strip them).
func IsCombiningMark(r rune) bool {
	// No combining mark (Mn/Mc/Me) exists below U+0300, so ASCII and Latin-1 skip the three range-table binary
	// searches — this runs per rune on the hot path.
	if r < 0x0300 {
		return false
	}

	return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me)
}
