//go:generate go run ./ucd/cmd/gen_asciifold mangling asciifold_table 15.0.0
//go:generate go run ./ucd/cmd/gen_asciifold mangling asciifold_table 17.0.0

package mangling

import (
	"maps"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-openapi/swag/mangling/v2/internal/tokens"
	"github.com/go-openapi/swag/mangling/v2/numbers"
	"github.com/go-openapi/swag/mangling/v2/runewords"
)

// ToASCII transforms a string to plain ASCII.
//
// Latin diacritics are folded (café → cafe), combining marks stripped,
// and any remaining non-ASCII rune is replaced by its phonetic Unicode-name word (π → pi, 😀 → grinning face),
// space-separated so it reads as words.
//
// Non-ASCII decimal digits fold to their ASCII value (٧ → 7), and numeral runes render as a plain number (½ → 0.5).
//
// Runes with no known word (CJK ideographs, decorative symbols) are dropped.
//
// This works best for European languages; it falls back to [RuneShortName] for other scripts and emoji.
func ToASCII[T ~string | ~[]byte](s T) string {
	in := string(s)
	if isASCII(in) {
		return in
	}

	var b strings.Builder
	b.Grow(len(in))
	for _, r := range in {
		switch {
		case r < utf8.RuneSelf:
			b.WriteRune(r)
		case tokens.IsCombiningMark(r):
			// strip
		default:
			if f, ok := asciiFold[r]; ok {
				b.WriteString(f)
			} else if d, ok := asciiDigit(r); ok {
				b.WriteByte(d) // non-ASCII decimal digit (Nd) → its ASCII value ("٧" → "7"), like the pipeline
			} else if v, ok := numbers.RuneNumber(r); ok {
				b.WriteByte(' ')
				b.WriteString(formatNumeral(v)) // numeral rune → plain number ("½" → "0.5"), not wording
				b.WriteByte(' ')
			} else if w, ok := runewords.Word(r); ok {
				b.WriteByte(' ')
				b.WriteString(w)
				b.WriteByte(' ')
			} // else: dropped
		}
	}

	return strings.Join(strings.Fields(b.String()), " ")
}

// RuneToASCII returns the plain-ASCII equivalent of a single rune bearing a diacritic (é → "e", ñ → "n"),
// a non-ASCII decimal digit folded to its ASCII value (٧ → "7"), the rune itself if already ASCII,
// or "" if it has no ASCII folding (non-Latin letters, symbols, emoji).
//
// Use [RuneShortName] for those.
// Combining marks fold to "".
func RuneToASCII[T ~rune | ~byte](r T) string {
	c := rune(r)
	if c < utf8.RuneSelf {
		return string(c)
	}
	if s, ok := asciiFold[c]; ok {
		return s
	}
	if d, ok := asciiDigit(c); ok {
		return string(d)
	}

	return ""
}

// RuneShortName returns a lowercase phonetic word for a rune with no ASCII folding.
//
// The word is a distinctive Unicode-name fragment (π → "pi", 😀 → "grinning face", ж → "zhe").
// ASCII runes are returned as-is.
//
// NOTE: runes the mangler elides (CJK ideographs, combining marks, decorative symbols) return "".
func RuneShortName[T ~rune | ~byte](r T) string {
	c := rune(r)
	if c < utf8.RuneSelf {
		return string(c)
	}
	if w, ok := runewords.Word(c); ok {
		return w
	}

	return ""
}

// foldASCII is the ASCII-folding stage.
//
// It rewrites each token that contains foldable non-ASCII runes into its ASCII form (Latin diacritics folded via
// [asciiFold], combining marks stripped).
//
// It runs between segmentation and assembly when folding is enabled (off in the base [Mangler], on in the [GoMangler]).
//
// Pure-ASCII tokens, and tokens whose non-ASCII runes are non-foldable (e.g. CJK — a future rune-name concern), are
// left untouched, so nothing allocates for them.
func (m Mangler) foldASCII(t *tokens.Tokens) {
	for i := range t.Len() {
		runes, override := t.Span(i)
		if override != "" {
			continue // already rewritten by an earlier stage
		}

		if folded, ok := foldToASCII(runes); ok {
			t.Rewrite(i, folded)
		}
	}
}

// foldToASCII returns the ASCII-folded form of runes and whether any folding happened.
//
// It allocates only when something is actually folded (detected in a cheap first pass).
func foldToASCII(runes []rune) (string, bool) {
	needsFold := false
	for _, r := range runes {
		if r >= utf8.RuneSelf && foldable(r) {
			needsFold = true

			break
		}
	}

	if !needsFold {
		return "", false
	}

	var b strings.Builder
	b.Grow(len(runes))

	for _, r := range runes {
		switch {
		case r < utf8.RuneSelf:
			_, _ = b.WriteRune(r)
		case tokens.IsCombiningMark(r):
			// strip
		default:
			if s, ok := asciiFold[r]; ok {
				_, _ = b.WriteString(s)
			} else {
				_, _ = b.WriteRune(r) // non-foldable (e.g. CJK): left for the future rune-name stage
			}
		}
	}

	return b.String(), true
}

func foldable(r rune) bool {
	if _, ok := asciiFold[r]; ok {
		return true
	}

	return tokens.IsCombiningMark(r)
}

// asciiDigit maps a decimal-digit rune (category Nd, any script: ASCII, Arabic-Indic ٧, Devanagari ०, Thai ๗,
// fullwidth ７, …) to its ASCII digit byte '0'–'9', and reports whether r is a decimal digit.
//
// No table is needed: Unicode lays out every script's digits as 10 consecutive codepoints (a hard invariant of Nd), so
// the value is r minus its unicode.Nd block start.
// Only the ~64 Nd blocks are scanned, and only for non-ASCII runes (a cold path).
func asciiDigit(r rune) (byte, bool) {
	if r >= '0' && r <= '9' {
		return byte(r), true
	}
	if r < 0x0660 {
		// No non-ASCII decimal digit (Nd) exists below U+0660 (Arabic-Indic), so skip the block scan for the common
		// Latin/Greek/Cyrillic range.
		return 0, false
	}
	for _, rg := range unicode.Nd.R16 {
		lo, hi := rune(rg.Lo), rune(rg.Hi)
		if lo <= r && r <= hi {
			return byte('0' + (r-lo)%10), true
		}
	}
	for _, rg := range unicode.Nd.R32 {
		lo, hi := rune(rg.Lo), rune(rg.Hi) //nolint:gosec // unicode.Nd range bounds are valid codepoints (<= MaxRune)
		if lo <= r && r <= hi {
			return byte('0' + (r-lo)%10), true
		}
	}

	return 0, false
}

// isAllCombiningMarks reports whether every rune is a combining mark (so the token renders to nothing once marks are
// stripped).
//
// An empty slice counts as all-marks (also renders to nothing).
func isAllCombiningMarks(runes []rune) bool {
	for _, r := range runes {
		if !tokens.IsCombiningMark(r) {
			return false
		}
	}

	return true
}

// DefaultSymbolWords returns a copy of the built-in symbol-verbalization set (rune → word, e.g. '@' → "at").
//
// Use it as a starting point to build a custom set for [WithSymbolWords] when you want wholesale control rather than a
// small overlay. The returned map is a fresh copy — mutating it does not affect the mangler.
func DefaultSymbolWords() map[rune]string {
	return maps.Clone(defaultSymbolWords)
}

// defaultSymbolWords maps a single symbol rune to the word it verbalizes to (e.g. "@" => "at", "!" => "bang").
//
// This is the default data for the symbol verbalization policy: when a target verbalizes a symbol rather than dropping
// it, this table supplies the word. It is deliberately narrow — only symbols that read meaningfully as a word.
//
// The tokenizer treats a rune in this map as a symbol token (not a separator), and the assembler renders it with its
// word under the verbalize policy.
//
// Multi-rune operators (!=, <=, ->, …) and the comparison glyphs (<, >, ≠, ≤, ≥, …) are NOT here: they verbalize as
// multi-word phrases, which must be expanded before segmentation (see [operatorWords] / [expandOperators]) so they
// re-segment and case per word.
//
// Also explicitly NOT included (handled elsewhere):
//   - separators and whitespace (space, and — depending on config — '-' '_' '.'): consumed by segmentation;
//   - structural/grouping punctuation (brackets, braces, parens, quotes): default policy drops them;
//   - letters with diacritics: folded to ASCII via [asciiFold].
var defaultSymbolWords = map[rune]string{
	// operators & markers (ASCII)
	'@':  "at",
	'&':  "and",
	'#':  "hash",
	'%':  "percent",
	'+':  "plus",
	'=':  "equal", // uninflected, consistent with "==" (operatorWords); the symbol words are labels, not conjugated verbs
	'*':  "star",
	'/':  "slash",
	'\\': "backslash",
	'|':  "pipe",
	'~':  "tilde",
	'^':  "caret",
	'!':  "bang",
	'?':  "question",
	'$':  "dollar",
	'.':  "dot", // e.g. spelled decimals: "one dot two" (verbalize vs. elide is the target's symbol policy)

	// currency & misc symbols worth a word (non-ASCII)
	'€': "euro",
	'£': "pound",
	'¥': "yen",
	'¢': "cent",
	'¤': "currency", // generic currency sign
	'₹': "rupee",    // our word, not Unicode's ("INDIAN RUPEE SIGN")
	'₩': "won",
	'₽': "ruble",
	'₿': "bitcoin",
	'©': "copyright",
	'®': "registered",
	'™': "trademark",
	'№': "numero",
	'§': "section",
	'¶': "paragraph",
	'°': "degree",
	'µ': "micro",
	'×': "times",
	'÷': "divide",
	'±': "plusminus",
}
