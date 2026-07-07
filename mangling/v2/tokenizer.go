package mangling

import (
	"iter"
	"unicode"
)

// Tokenizer splits an UTF-8 string into tokens along opinionated segmentation rules (§4.2).
//
// # Token boundaries
//
// A run of runes becomes a token; a boundary falls at any of:
//
//   - one or more consecutive separators (by default: whitespace, structural punctuation and any
//     non-printable rune — see [defaultTokenSeparator]); separators are elided, not emitted,
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
// Script/Unicode-category change (§4.2 signal #4) is not yet implemented.
//
// Separators may be customized by injecting a predicate with option [WithTokenSeparator].
type Tokenizer struct {
	tokenOptions
}

// Tokenize splits a string into its tokens, materialized as strings.
//
// This is a convenience surface (it allocates a string per token).
// The mangling pipeline works on the zero-copy [Tokens] model directly.
func (m Tokenizer) Tokenize(in string) iter.Seq[string] {
	return func(yield func(string) bool) {
		t := borrowTokens(in)
		defer t.redeem()
		m.segment(&t)

		for i := range t.Len() {
			if !yield(t.Text(i)) {
				return
			}
		}
	}
}

// segment fills t with the tokens of its shared []rune, implementing the boundary rules above.
func (m Tokenizer) segment(t *Tokens) {
	sep := m.separator
	if sep == nil {
		sep = defaultTokenSeparator
	}

	runes := t.runes.Slice()
	n := len(runes)

	runStart := -1 // -1 means "no active run"
	var runKind Kind

	flush := func(end int) {
		if runStart < 0 {
			return
		}
		casing := CasingMixed
		if runKind == KindWord {
			casing = classifyCasing(runes[runStart:end])
		}
		t.push(runStart, end, runKind, casing)
		runStart = -1
	}

	for i := 0; i < n; i++ {
		r := runes[i]

		switch classify(r, sep) {
		case classSeparator:
			flush(i) // elided, not emitted

		case classSymbol:
			flush(i)
			t.push(i, i+1, KindSymbol, CasingMixed)

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
				}
				// otherwise: same case / single-upper / caseless → extend the run
			}
		}
	}
	flush(n)
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
	case unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me):
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

// classifyCasing derives a word run's [Casing] from its runes (marks/caseless runes are ignored).
func classifyCasing(runes []rune) Casing {
	var upper, lower int
	for _, r := range runes {
		switch {
		case unicode.IsUpper(r):
			upper++
		case unicode.IsLower(r):
			lower++
		}
	}

	switch {
	case upper == 0 && lower == 0:
		return CasingMixed // caseless content
	case lower == 0:
		return CasingUpper // all-caps: HTTP, A
	case upper == 0:
		return CasingLower // http
	case upper == 1 && unicode.IsUpper(runes[0]):
		return CasingTitle // Http (only the first rune is upper)
	default:
		return CasingMixed // hTtP
	}
}

// Transform is a pipeline stage: it mutates the token model in place.
//
// It replaces the retired string-based transformer tier — stages see position, kind and casing, and may split/merge
// tokens.
// See [Tokens] (token.go / tokens.go).
type Transform func(*Tokens)
