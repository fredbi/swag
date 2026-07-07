package mangling_test

import (
	"go/token"
	"testing"

	mangling "github.com/go-openapi/swag/mangling/v2"
)

// FuzzGoIdent asserts the load-bearing contract.
//
// For ANY input, the Go identifier producers emit a valid Go identifier
// (go/token.IsIdentifier — non-empty, letter/underscore start, letters/digits/ underscore body, not a keyword)
// with the expected export visibility.
//
// It is a robustness guard ("all-weather": never crash, always produce a compilable identifier), not a golden-output
// test.
//
// Idempotency (f(f(x)) == f(x)) is deliberately NOT asserted: casing is lossy — it discards word boundaries — so "A
// A" -> "AA" (two words) and re-mangling the all-caps "AA" yields "Aa" (one word).
//
// That is inherent to PascalCase/camelCase, not a defect; only validity is a genuine invariant.
func FuzzGoIdent(f *testing.F) {
	for _, s := range identSeeds {
		f.Add(s)
	}

	// Both folding modes must uphold the valid-identifier contract.
	// Export visibility, however, is only guaranteed with folding on: with folding off a caseless-script first letter
	// (e.g. CJK, preserved) is neither exported nor unexported by Go's rules, so token.IsExported is not a sound check
	// there.
	modes := []struct {
		name          string
		g             mangling.GoMangler
		checkExported bool
	}{
		{"folding", mangling.MakeGoMangler(), true},
		{"raw", mangling.MakeGoMangler(mangling.WithManglerOptions(mangling.WithASCIIFolding(false))), false},
	}

	f.Fuzz(func(t *testing.T, in string) {
		for _, m := range modes {
			producers := []struct {
				name     string
				mangle   func(string) string
				exported bool
			}{
				{"IdentExported", m.g.IdentExported, true},
				{"IdentUnexported", m.g.IdentUnexported, false},
				{"ConstName", func(s string) string { return m.g.ConstName(s) }, true},
			}

			for _, p := range producers {
				out := p.mangle(in)

				if !token.IsIdentifier(out) {
					t.Errorf("[%s] %s(%q) = %q is not a valid Go identifier", m.name, p.name, in, out)

					continue
				}
				if m.checkExported && token.IsExported(out) != p.exported {
					t.Errorf("[%s] %s(%q) = %q has wrong export visibility (want exported=%v)", m.name, p.name, in, out, p.exported)
				}
			}
		}
	})
}

// identSeeds spans the categories that drive the mangler's branches.
//
// empty / separators-only, ASCII words, Go keywords & builtins, Latin diacritics, non-Latin scripts,
// RTL, numeral runes (No/Nl), leading digits, emoji, combining marks, and control/invalid bytes.
//
// Several are the exact cases that exposed the leading-numeral contract hole, kept here as permanent regressions.
var identSeeds = []string{
	"", " ", "_", "___", "---", ".", "@#$", "\x00\x01\x02",
	"hello world", "HTTPServer", "getHTTPResponse", "findThingByID",
	"type", "range", "func", "append", "make", "any", // keywords & builtins
	"café résumé", "naïve", "Żaba", "Éric", "straße",
	"日本語", "很长的中文字符串", "مرحبا العالم", "Ω Δ π", "αβγ", "ЖenЯ",
	"½ place", "²power", "⅐", "Ⅶ legions", "①②③", "3.14 x", "2nd", "9lives", "007",
	"٧", "٧ place", "café٧", "०१२", "๗ items", "７", // non-ASCII Nd decimal digits (Arabic/Devanagari/Thai/fullwidth)
	"😀 grin", "👍 emoji", "❤ heart",
	"áb", "́leading mark", // combining marks
	"  leading  spaces  ", "trailing---", "MixedCASE123!@#",
	// value-like inputs — representative of ConstName (enum members): numbers, signs, fractions, units, media types,
	// symbols, statuses.
	"active", "in progress", "read-only", "200", "-5", "0.25", "3.14", "1e9", "status 404",
	"application/json", "50%", "€100", "N/A", "@id", "#tag", "v1.2.3", "2xx", "temp -40°C", "½ off",
}
