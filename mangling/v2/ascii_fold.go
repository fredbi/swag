package mangling

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-openapi/swag/mangling/v2/numbers"
	"github.com/go-openapi/swag/mangling/v2/runewords"
)

// ToASCII transforms a string to plain ASCII.
//
// Latin diacritics are folded (café → cafe), combining marks stripped,
// and any remaining non-ASCII rune is replaced by its phonetic Unicode-name word (π → pi, 😀 → grinning face),
// space-separated so it reads as words.
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
		case isCombiningMark(r):
			// strip
		default:
			if f, ok := asciiFold[r]; ok {
				b.WriteString(f)
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

// RuneToASCII returns the plain-ASCII equivalent of a single rune bearing a diacritic (é → "e", ñ → "n"), the
// rune itself if already ASCII, or "" if it has no ASCII folding (non-Latin letters, symbols, emoji).
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
func (m Mangler) foldASCII(t *tokens) {
	for i := range t.Len() {
		runes, override := t.span(i)
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
		case isCombiningMark(r):
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

	return isCombiningMark(r)
}

func isCombiningMark(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me)
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
		if !isCombiningMark(r) {
			return false
		}
	}

	return true
}

// asciiFold maps a Latin letter bearing a diacritic (or a distinct Latin letter such as æ, ß, þ) to its plain ASCII
// equivalent, preserving case.
//
// It is the data that supports [Ascii] / [ToASCII].
//
// Scope: European Latin scripts (Latin-1 Supplement, Latin Extended-A, a few Extended-B).
//
// This is diacritic *folding* — strip the accent, keep the base letter (ü→u, not the German ü→ue
// transliteration).
//
// Distinct letters that have no single-rune ASCII base fold to their conventional digraph (æ→ae, œ→oe, ß→ss,
// þ→th, ð→d).
//
// NOT covered here (by design): symbols and punctuation — see [defaultSymbolWords]; and non-Latin scripts (Greek,
// Cyrillic, CJK, …), which fall back to the phonetic rune name (see [RuneShortName]).
var asciiFold = map[rune]string{
	// A
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a", 'ǎ': "a",
	'À': "A", 'Á': "A", 'Â': "A", 'Ã': "A", 'Ä': "A", 'Å': "A", 'Ā': "A", 'Ă': "A", 'Ą': "A", 'Ǎ': "A",
	// AE (ligature / distinct letter)
	'æ': "ae", 'Æ': "AE",
	// C
	'ç': "c", 'ć': "c", 'ĉ': "c", 'ċ': "c", 'č': "c",
	'Ç': "C", 'Ć': "C", 'Ĉ': "C", 'Ċ': "C", 'Č': "C",
	// D (incl. đ d-bar and ð eth)
	'ď': "d", 'đ': "d", 'ð': "d",
	'Ď': "D", 'Đ': "D", 'Ð': "D",
	// E (incl. ə schwa)
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ĕ': "e", 'ė': "e", 'ę': "e", 'ě': "e", 'ə': "e",
	'È': "E", 'É': "E", 'Ê': "E", 'Ë': "E", 'Ē': "E", 'Ĕ': "E", 'Ė': "E", 'Ę': "E", 'Ě': "E",
	// G
	'ĝ': "g", 'ğ': "g", 'ġ': "g", 'ģ': "g",
	'Ĝ': "G", 'Ğ': "G", 'Ġ': "G", 'Ģ': "G",
	// H
	'ĥ': "h", 'ħ': "h",
	'Ĥ': "H", 'Ħ': "H",
	// I (incl.
	// Turkish ı dotless and İ dotted)
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ĩ': "i", 'ī': "i", 'ĭ': "i", 'į': "i", 'ı': "i",
	'Ì': "I", 'Í': "I", 'Î': "I", 'Ï': "I", 'Ĩ': "I", 'Ī': "I", 'Ĭ': "I", 'Į': "I", 'İ': "I",
	// J
	'ĵ': "j", 'Ĵ': "J",
	// K
	'ķ': "k", 'Ķ': "K",
	// L (incl. ł l-stroke)
	'ĺ': "l", 'ļ': "l", 'ľ': "l", 'ŀ': "l", 'ł': "l",
	'Ĺ': "L", 'Ļ': "L", 'Ľ': "L", 'Ŀ': "L", 'Ł': "L",
	// N (incl. ŋ eng)
	'ñ': "n", 'ń': "n", 'ņ': "n", 'ň': "n", 'ŋ': "n",
	'Ñ': "N", 'Ń': "N", 'Ņ': "N", 'Ň': "N", 'Ŋ': "N",
	// O (incl. ø o-slash)
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ŏ': "o", 'ő': "o",
	'Ò': "O", 'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O", 'Ø': "O", 'Ō': "O", 'Ŏ': "O", 'Ő': "O",
	// OE (ligature)
	'œ': "oe", 'Œ': "OE",
	// R
	'ŕ': "r", 'ŗ': "r", 'ř': "r",
	'Ŕ': "R", 'Ŗ': "R", 'Ř': "R",
	// S (incl. ș s-comma)
	'ś': "s", 'ŝ': "s", 'ş': "s", 'š': "s", 'ș': "s",
	'Ś': "S", 'Ŝ': "S", 'Ş': "S", 'Š': "S", 'Ș': "S",
	// SS (sharp s)
	'ß': "ss", 'ẞ': "SS",
	// T (incl. ț t-comma)
	'ţ': "t", 'ť': "t", 'ŧ': "t", 'ț': "t",
	'Ţ': "T", 'Ť': "T", 'Ŧ': "T", 'Ț': "T",
	// TH (thorn)
	'þ': "th", 'Þ': "Th",
	// U
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ũ': "u", 'ū': "u", 'ŭ': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'Ù': "U", 'Ú': "U", 'Û': "U", 'Ü': "U", 'Ũ': "U", 'Ū': "U", 'Ŭ': "U", 'Ů': "U", 'Ű': "U", 'Ų': "U",
	// W
	'ŵ': "w", 'Ŵ': "W",
	// Y
	'ý': "y", 'ÿ': "y", 'ŷ': "y",
	'Ý': "Y", 'Ÿ': "Y", 'Ŷ': "Y",
	// Z
	'ź': "z", 'ż': "z", 'ž': "z",
	'Ź': "Z", 'Ż': "Z", 'Ž': "Z",
}

// defaultSymbolWords maps a symbol rune to the word it verbalizes to (e.g. "@" => "at", "!" => "bang").
//
// This is the default data for the symbol verbalization policy: when a target chooses to *verbalize* a symbol rather
// than drop it, this table supplies the word.
// It is deliberately narrow — only symbols that read meaningfully as a word.
//
// Explicitly NOT included (handled elsewhere, not by verbalization):
//   - separators and whitespace (space, and — depending on config — '-' '_' '.'): consumed by segmentation;
//   - structural/grouping punctuation (brackets, braces, parens, quotes): default policy drops them;
//   - letters with diacritics: folded to ASCII via [asciiFold].
//
// The current [defaultTokenSeparator] treats all [unicode.IsPunct] as a separator, which is too wide — it would elide
// the very symbols listed here before they could be verbalized.
//
// Reconciling that (separator set vs symbol-word set) is a wiring concern, deferred.
//
// NOTE: prepared as data only; not yet wired into the pipeline.
var defaultSymbolWords = map[rune]string{
	// operators & markers (ASCII)
	'@':  "at",
	'&':  "and",
	'#':  "hash",
	'%':  "percent",
	'+':  "plus",
	'=':  "equals",
	'*':  "star",
	'/':  "slash",
	'\\': "backslash",
	'|':  "pipe",
	'~':  "tilde",
	'^':  "caret",
	'!':  "bang",
	'?':  "question",
	'<':  "less",
	'>':  "greater",
	'$':  "dollar",
	'.':  "dot", // e.g. spelled decimals: "one dot two" (verbalize vs. elide is the target's symbol policy)

	// currency & misc symbols worth a word (non-ASCII)
	'€': "euro",
	'£': "pound",
	'¥': "yen",
	'¢': "cent",
	'©': "copyright",
	'®': "registered",
	'™': "trademark",
	'§': "section",
	'¶': "paragraph",
	'°': "degree",
	'µ': "micro",
	'×': "times",
	'÷': "divide",
	'±': "plusminus",
}
