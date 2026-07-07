package mangling

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// foldASCII is the ASCII-folding stage: it rewrites each token that contains foldable non-ASCII runes into its
// ASCII form (Latin diacritics folded via [asciiFold], combining marks stripped).
//
// It runs between segmentation and assembly when folding is enabled (off in the base [Mangler], on in the [GoMangler]).
//
// Pure-ASCII tokens, and tokens whose non-ASCII runes are non-foldable (e.g. CJK — a future rune-name concern),
// are left untouched, so nothing allocates for them.
func (m Mangler) foldASCII(t *Tokens) {
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
			b.WriteRune(r)
		case isCombiningMark(r):
			// strip
		default:
			if s, ok := asciiFold[r]; ok {
				b.WriteString(s)
			} else {
				b.WriteRune(r) // non-foldable (e.g. CJK): left for the future rune-name stage
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
