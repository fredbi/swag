package mangling

import (
	"fmt"
	"iter"
	"unicode"

	"github.com/go-openapi/swag/pools"
)

// Pooled backings for the zero-copy token model, reused across [Mangler.Transform] calls.
//
// The token slice is the churny one; the rune slice holds the single shared copy of the input.
var (
	tokenSlicePool = pools.NewPoolSlice[token]()
	runeSlicePool  = pools.NewPoolSlice[rune]()
)

// tokenizer splits an UTF-8 string into tokens along opinionated segmentation rules.
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
//
// A script change between letters (e.g. Latin↔Cyrillic) is intentionally not a boundary yet: with ASCII folding on,
// non-Latin runes are romanized or elided before this point, so it rarely matters.
//
// Separators may be customized by injecting a predicate with option [WithTokenSeparator].
type tokenizer struct {
	tokenOptions
}

// Tokenize splits a string into its tokens, materialized as strings.
//
// This is a convenience surface (it allocates a string per token).
// The mangling pipeline works on the zero-copy [tokens] model directly.
func (m tokenizer) Tokenize(in string) iter.Seq[string] {
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
func (m tokenizer) segment(t *tokens) {
	sep := m.separator
	if sep == nil {
		sep = defaultTokenSeparator
	}

	runes := t.runes.Slice()
	n := len(runes)

	runStart := -1 // -1 means "no active run"
	var runKind tokenKind

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
			t.push(i, i+1, kindSymbol)

		case classMark:
			if runStart < 0 {
				// orphan/leading mark: keep it in a word run so nothing is silently lost (the fold stage will strip it).
				runStart, runKind = i, kindWord
			}
			// otherwise it attaches to the current run (extends on flush)

		case classDigit:
			switch {
			case runStart < 0:
				runStart, runKind = i, kindNumber
			case runKind == kindWord:
				flush(i) // letter↔digit boundary
				runStart, runKind = i, kindNumber
			}
			// else: extend the number run

		case classLetter:
			switch {
			case runStart < 0:
				runStart, runKind = i, kindWord
			case runKind == kindNumber:
				flush(i) // digit↔letter boundary
				runStart, runKind = i, kindWord
			default:
				// letter continuing a word run: check case alternance
				prev, cur := runeCase(runes[i-1]), runeCase(r)
				switch {
				case prev == caseLower && cur == caseUpper:
					// fooBar => foo | Bar
					flush(i)
					runStart, runKind = i, kindWord
				case prev == caseUpper && cur == caseLower && i-1 > runStart && runeCase(runes[i-2]) == caseUpper:
					// Upper-run(>=2) → lower: HTTPServer => HTTP | Server (boundary before the last upper)
					flush(i - 1)
					runStart, runKind = i-1, kindWord
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

// token is a zero-copy view into the shared []rune of a [tokens] value.
//
// A half-open span plus the classification computed by the scanner.
//
// It is internal: transforms reach token data only through [tokens]' index-based methods, so the struct can evolve
// without touching the public API.
type token struct {
	start, end int       // half-open span [start,end) into tokens.runes
	kind       tokenKind // word | number | symbol | initialism
	override   string    // rewritten content; empty unless a stage replaced the span
}

// tokenKind classifies a token produced by segmentation.
//
// The tokenizer emits [kindWord], [kindNumber] and [kindSymbol].
// [kindInitialism] is set later by the initialism overlay, never by the tokenizer.
type tokenKind uint8

const (
	kindWord       tokenKind = iota // a run of letters
	kindNumber                      // a run of decimal digits (Nd)
	kindSymbol                      // a single non-letter, non-digit, non-separator rune (@, #, …)
	kindInitialism                  // retagged by the initialism overlay (HTTP, JSON, …)
)

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

// tokens is the mutable, pooled token model: a slice of [token] spans over one shared []rune (the only full copy of the
// input).
//
// Pipeline stages mutate it in place; strings are materialized only at assembly.
// It is borrowed from a pool for the duration of one mangling and released with [tokens.redeem]; it must not be
// retained afterwards.
//
// The internal surface is index-based (the [token] struct stays private): a stage reads with Len/Text/kindOf and edits
// with Rewrite.
// A public stage-injection API may expose a version of this later; for now the model is unexported.
type tokens struct {
	runes *pools.Slice[rune]
	toks  *pools.Slice[token]

	// count is the logical number of tokens.
	//
	// It equals the pooled slice length after segmentation, but a stage that merges tokens in place (e.g. the initialism
	// overlay) shrinks it below the slice length, so only toks[:count] are live.
	count int

	releaseRunes func()
	releaseToks  func()
}

// borrowTokens borrows a tokens and loads the input as one shared []rune.
//
// It returns the wrapper by value so it stays on the caller's stack (no heap alloc): the pooled slices and their redeem
// closures are cached by pools, so nothing here allocates.
func borrowTokens(in string) tokens {
	// len(in) bytes is an upper bound on the rune count, so the pre-grown slice never reallocates.
	runeSlice, releaseRunes := runeSlicePool.BorrowWithSizeAndRedeem(len(in))
	for _, r := range in {
		runeSlice.Append(r)
	}
	tokSlice, releaseToks := tokenSlicePool.BorrowWithRedeem()

	return tokens{
		runes:        runeSlice,
		toks:         tokSlice,
		releaseRunes: releaseRunes,
		releaseToks:  releaseToks,
	}
}

// --- read API ---

// Len is the number of live tokens.
func (t *tokens) Len() int { return t.count }

// Text returns the content of token i: its rewritten override if set, else its rune span.
func (t *tokens) Text(i int) string {
	tk := t.toks.Slice()[i]
	if tk.override != "" {
		return tk.override
	}

	return string(t.runes.Slice()[tk.start:tk.end])
}

// Deferred token capabilities — no current caller, kept commented as a memo rather than carried as dead code.
// Uncomment and test when a stage actually needs one.
//
// 	// All ranges over the tokens' rendered text by index (read-only).
// 	func (t *tokens) All() iter.Seq2[int, string] { ... }
//
// 	// SetKind retags token i — e.g. the initialism overlay marks a token kindInitialism.
// 	func (t *tokens) SetKind(i int, kind tokenKind) { t.toks.Slice()[i].kind = kind }
//
// 	// Split divides token i at offset at into two adjacent tokens (sub-token initialism, IDS → ID + S).
// 	func (t *tokens) Split(i, at int) { ... }
//
// 	// Merge folds tokens [i, j] into one (multi-token initialism merge, IPv4/UTF8).
// 	func (t *tokens) Merge(i, j int) { ... }

// --- write API (mutating an element in place is safe; growing goes through the pool wrapper) ---.

// Rewrite replaces the rendered content of token i (transliteration, inflection, verbalization).
func (t *tokens) Rewrite(i int, s string) {
	t.toks.Slice()[i].override = s
}

// kindOf returns the kind of token i.
func (t *tokens) kindOf(i int) tokenKind { return t.toks.Slice()[i].kind }

// runeLen is the number of runes in the shared input (a size hint for assembly).
func (t *tokens) runeLen() int { return t.runes.Len() }

// span returns token i's raw rune span (a view into the shared slice — no copy) and its override (empty unless a
// transform rewrote it).
func (t *tokens) span(i int) ([]rune, string) {
	tk := t.toks.Slice()[i]

	return t.runes.Slice()[tk.start:tk.end], tk.override
}

// redeem returns the pooled backings.
//
// The tokens must not be used afterwards.
func (t *tokens) redeem() {
	t.releaseToks()
	t.releaseRunes()
}

// push appends a token spanning [start,end) with its classification.
//
// Scanner-only.
func (t *tokens) push(start, end int, kind tokenKind) {
	t.toks.Append(token{start: start, end: end, kind: kind})
	t.count++
}

// defaultTokenSeparator reports whether a rune is a token separator — a rune that is *elided* (dropped, never
// emitted) and marks a boundary between tokens.
//
// It implements bucket 3 of the segmentation classification (see also [defaultSymbolWords]):
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
// NOTE: this is the intended default predicate; it is not yet wired into a working tokenizer.
func defaultTokenSeparator(r rune) bool {
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
