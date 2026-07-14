package mangling

import (
	"strings"
	"unicode"

	"github.com/go-openapi/swag/mangling/v2/internal/tokens"
)

// DefaultInitialisms returns the built-in set of initialisms recognized when Go initialisms are enabled.
//
// These are common acronyms (API, HTTP, ID, JSON, ...) that the Go mangler keeps fully upper-cased in
// exported identifiers, plus a few mixed-case exceptions (IPv4, IPv6). Use it as a starting point to
// extend or override the set through the initialism options.
func DefaultInitialisms() []string {
	return []string{
		"ACL",
		"API",
		"ASCII",
		"CPU",
		"CSS",
		"DNS",
		"EOF",
		"GUID",
		"HTML",
		"HTTPS",
		"HTTP",
		"ID",
		"IP",
		"IPv4", // prefer the mixed case outcome IPv4 over the capitalized IPV4
		"IPv6", // prefer the mixed case outcome IPv6 over the capitalized IPV6
		"JSON",
		"LHS",
		"OAI",
		"QPS",
		"RAM",
		"RHS",
		"RPC",
		"SLA",
		"SMTP",
		"SQL",
		"SSH",
		"TCP",
		"TLS",
		"TTL",
		"UDP",
		"UI",
		"UID",
		"UUID",
		"URI",
		"URL",
		"UTF8",
		"VM",
		"XML",
		"XMPP",
		"XSRF",
		"XSS",
	}
}

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
// It reads token bounds and runes through the [tokens.Tokens] index API (no access to the private token struct).
// Terminals that fall mid-token are ignored (no sub-token prefix matching).
// A pluralized terminal is accepted only when its suffix reads lowercase in the input (so "IDs" pluralizes but "IDS"
// does not).
func (t *initialismTrie) match(tk *tokens.Tokens, runes []rune, r, n int) (string, int) {
	node := &t.root
	bestCanonical := ""
	bestSpan := 0

	pos, _ := tk.Bounds(r)
	for tokIdx := r; tokIdx < n; tokIdx++ {
		start, end := tk.Bounds(tokIdx)
		if start != pos {
			break // not contiguous (an elided separator sits between): initialisms don't cross separators
		}

		for i := start; i < end; i++ {
			child := node.children[unicode.ToLower(runes[i])]
			if child == nil {
				return bestCanonical, bestSpan
			}
			node = child

			if i == end-1 && node.canonical != "" && (!node.plural || unicode.IsLower(runes[i])) {
				// terminal aligned with a token boundary (and, if plural, a lowercase suffix)
				bestCanonical = node.canonical
				bestSpan = tokIdx - r + 1
			}
		}

		pos = end
	}

	return bestCanonical, bestSpan
}

// applyInitialisms is the Go initialism overlay: a single forward compaction pass that rewrites runs of tokens forming
// a known initialism — whole-token (HTTP), a contiguous multi-token break-run (ipv4 → [ipv,4]), or a
// lowercase-plural (IDs → [I,Ds]) — into one [kindInitialism] token carrying the canonical casing.
//
// Merges shrink the slice in place; assembly then renders the canonical form (with the leading-unexported lowercasing
// rule).
func (g GoMangler) applyInitialisms(t *tokens.Tokens) {
	if g.trie == nil {
		return
	}

	runes := t.Runes()
	n := t.Len()

	// The overlay owns the in-place forward compaction; this callback supplies only the matching *policy*: at each
	// position, the longest initialism run (if any) and the canonical casing to emit for it.
	t.Overlay(func(from int) (int, tokens.Kind, string) {
		canonical, span := g.trie.match(t, runes, from, n)
		if span == 0 {
			return 0, 0, ""
		}

		return span, tokens.KindInitialism, canonical
	})
}
