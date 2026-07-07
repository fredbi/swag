package mangling

import (
	"strings"
	"unicode"
)

// initialismTrie indexes the known initialisms (and their pluralized forms) for the overlay.
//
// It is keyed on lowercased runes, so matching is case-insensitive; the canonical casing to emit is stored at the
// terminal.
// Built once at [GoMangler] construction, read-only thereafter.
type initialismTrie struct {
	root initialismNode
}

type initialismNode struct {
	children  map[rune]*initialismNode
	canonical string // non-empty at a terminal: the canonical casing to emit
	plural    bool   // terminal is a pluralized form (accepted only when the suffix reads lowercase)
}

func buildInitialismTrie(initialisms []string) *initialismTrie {
	t := &initialismTrie{}
	set := toSet(initialisms)

	for _, ini := range initialisms {
		t.add(ini, ini, false)
		if isPluralizable(ini, set) {
			t.add(ini+"s", ini+"s", true) // canonical plural = base + lowercase "s"
		}
	}

	return t
}

func (t *initialismTrie) add(key, canonical string, plural bool) {
	node := &t.root
	for _, r := range key {
		lr := unicode.ToLower(r)
		if node.children == nil {
			node.children = make(map[rune]*initialismNode)
		}
		child := node.children[lr]
		if child == nil {
			child = &initialismNode{}
			node.children[lr] = child
		}
		node = child
	}
	node.canonical = canonical
	node.plural = plural
}

// isPluralizable reports whether an initialism takes a simple "+s" plural.
//
// It is invariant (no plural entry) when it ends in S/s (DNS, HTTPS) or when key+"s"/"S" is itself an initialism (HTTP
// vs HTTPS — keeps "https" mapping to HTTPS, not a spurious plural of HTTP).
// Mirrors v1's precompute.
func isPluralizable(ini string, set map[string]struct{}) bool {
	if strings.HasSuffix(strings.ToUpper(ini), "S") {
		return false
	}
	if _, ok := set[ini+"s"]; ok {
		return false
	}
	if _, ok := set[ini+"S"]; ok {
		return false
	}

	return true
}

// match walks the trie over the contiguous rune span starting at token r, returning the canonical form and token span
// of the longest initialism whose terminal lands on a **token boundary** (span 0 if none).
//
// Terminals that fall mid-token are ignored (no sub-token prefix matching).
// A pluralized terminal is accepted only when its suffix reads lowercase in the input (so "IDs" pluralizes but "IDS"
// does not).
func (t *initialismTrie) match(toks []token, runes []rune, r, n int) (string, int) {
	node := &t.root
	bestCanonical := ""
	bestSpan := 0

	pos := toks[r].start
	for tokIdx := r; tokIdx < n; tokIdx++ {
		tk := toks[tokIdx]
		if tk.start != pos {
			break // not contiguous (an elided separator sits between): initialisms don't cross separators
		}

		for i := tk.start; i < tk.end; i++ {
			child := node.children[unicode.ToLower(runes[i])]
			if child == nil {
				return bestCanonical, bestSpan
			}
			node = child

			if i == tk.end-1 && node.canonical != "" && (!node.plural || unicode.IsLower(runes[i])) {
				// terminal aligned with a token boundary (and, if plural, a lowercase suffix)
				bestCanonical = node.canonical
				bestSpan = tokIdx - r + 1
			}
		}

		pos = tk.end
	}

	return bestCanonical, bestSpan
}

// applyInitialisms is the Go initialism overlay: a single forward compaction pass that rewrites runs of tokens forming
// a known initialism — whole-token (HTTP), a contiguous multi-token break-run (ipv4 → [ipv,4]), or a
// lowercase-plural (IDs → [I,Ds]) — into one [KindInitialism] token carrying the canonical casing.
//
// Merges shrink the slice in place; assembly then renders the canonical form (with the leading-unexported lowercasing
// rule).
func (g GoMangler) applyInitialisms(t *Tokens) {
	if g.trie == nil {
		return
	}

	toks := t.toks.Slice()
	runes := t.runes.Slice()
	n := t.Len()

	w := 0
	for r := 0; r < n; {
		if canonical, span := g.trie.match(toks, runes, r, n); span > 0 {
			merged := toks[r]
			merged.end = toks[r+span-1].end
			merged.kind = KindInitialism
			merged.override = canonical
			toks[w] = merged
			w++
			r += span

			continue
		}

		toks[w] = toks[r]
		w++
		r++
	}

	t.count = w
}
