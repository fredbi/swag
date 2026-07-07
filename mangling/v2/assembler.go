package mangling

import (
	"fmt"
	"strings"
	"unicode"
)

// wordCasing is how a word token is cased at assembly.
type wordCasing uint8

const (
	casingAsIs  wordCasing = iota // leave as segmented
	casingLower                   // all lower: "http"
	casingTitle                   // first rune title, rest lower: "Http"
	casingUpper                   // all upper: "HTTP"
)

// symbolPolicy decides what assembly does with a symbol token.
//
// The zero value is symbolVerbalize: the neutral presets replace a known symbol with its word (e.g. "@" => "at").
// Value/Go targets may override to drop leading markers, etc.
type symbolPolicy uint8

const (
	symbolVerbalize symbolPolicy = iota // replace with its word (defaultSymbolWords), cased like a word (default)
	symbolDrop                          // elide the symbol
	symbolKeep                          // emit the symbol rune verbatim
)

// assemble renders the token stream into a string per the target recipe: casing × separator × symbol policy.
//
// The "first word" casing (which lets camelCase lowercase word 0) tracks the first *emitted word*, so a dropped leading
// symbol or a leading number doesn't consume it.
//
// Initialism casing, when present, is preserved from the token rather than re-cased.
func (m Mangler) assemble(t *Tokens, target TargetTransform) string {
	var b strings.Builder
	b.Grow(t.runeLen()) // one buffer alloc; ~exact for ASCII output

	firstWord := true
	wrote := false

	// writeSep emits the separator before a non-glued, non-first token.
	// Number tokens glue to the preceding token (no separator), so "simple 1" => "simple1", while a following word still
	// separates ("simple1_text2") and a leading number is separated by the word after it.
	writeSep := func(glue bool) {
		if wrote && !glue && target.separator != "" {
			b.WriteString(target.separator)
		}
		wrote = true
	}

	// nextCasing returns the casing for the next word and advances the first-word slot (camelCase lowercases word 0).
	nextCasing := func() wordCasing {
		if firstWord {
			firstWord = false

			return target.firstCasing
		}

		return target.restCasing
	}

	for i := range t.Len() {
		runes, override := t.span(i)

		switch t.Kind(i) {
		case KindWord:
			if override == "" && isAllCombiningMarks(runes) {
				continue // renders to nothing (marks are stripped) — don't emit a separator or consume the first-word slot
			}
			writeSep(false)
			writeCased(&b, runes, override, nextCasing())
		case KindInitialism:
			writeSep(false)
			// An initialism follows the target's casing *intent*, except title-casing preserves its canonical form:
			// lower → lowercase (snake, and leading in unexported → "httpGet"),
			// upper → uppercase,
			// title/as-is → canonical ("getHTTP", not "getHttp").
			c := target.restCasing
			if firstWord {
				c = target.firstCasing
			}
			firstWord = false
			if c == casingTitle {
				c = casingAsIs
			}
			writeCased(&b, runes, override, c)
		case KindNumber:
			writeSep(true) // glue to the preceding token
			writeCased(&b, runes, override, casingAsIs)
		case KindSymbol:
			switch target.symbolPolicy {
			case symbolDrop:
				// elide
			case symbolKeep:
				writeSep(false)
				writeCased(&b, runes, override, casingAsIs)
			default: // symbolVerbalize (the zero-value default)
				if w, ok := defaultSymbolWords[runes[0]]; ok {
					writeSep(false)
					writeStringCased(&b, w, nextCasing())
				}
			}
		default:
			panic(fmt.Errorf("internal error: invalid Kind: %v", t.Kind(i)))
		}
	}

	return b.String()
}

// writeCased writes a token's content (its rune span, or its override string) to b,
// applying the casing per rune — no per-token string is materialized.
func writeCased(b *strings.Builder, runes []rune, override string, c wordCasing) {
	if override != "" {
		writeStringCased(b, override, c)

		return
	}

	// Combining marks (Mn/Mc/Me) are never valid identifier characters, so strip them here regardless of
	// ASCII folding — the fold stage strips them too, but it only runs when folding is on. `first` tracks
	// the first *surviving* rune so title-casing lands on it after any leading mark is dropped.
	first := true
	for _, r := range runes {
		if isCombiningMark(r) {
			continue
		}

		switch c {
		case casingLower:
			r = unicode.ToLower(r)
		case casingUpper:
			r = unicode.ToUpper(r)
		case casingTitle:
			if first {
				r = unicode.ToTitle(r)
			} else {
				r = unicode.ToLower(r)
			}
		default: // casingAsIs
		}

		b.WriteRune(r)
		first = false
	}
}

// writeStringCased is writeCased for a string (override content, or a verbalized symbol word).
func writeStringCased(b *strings.Builder, s string, c wordCasing) {
	switch c {
	case casingLower:
		for _, r := range s {
			b.WriteRune(unicode.ToLower(r))
		}
	case casingUpper:
		for _, r := range s {
			b.WriteRune(unicode.ToUpper(r))
		}
	case casingTitle:
		first := true
		for _, r := range s {
			if first {
				b.WriteRune(unicode.ToTitle(r))
				first = false
			} else {
				b.WriteRune(unicode.ToLower(r))
			}
		}
	default: // casingAsIs
		b.WriteString(s)
	}
}
