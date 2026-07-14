// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"slices"
	"testing"

	"github.com/go-openapi/swag/mangling/v2/internal/tokens"
	"github.com/go-openapi/testify/v2/assert"
)

func TestWithGoReservedSuffix(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler(WithGoReservedSuffix("Kw"))
	assert.EqualT(t, "typeKw", g.IdentUnexported("type"))                // custom suffix
	assert.EqualT(t, "typeVar", MakeGoMangler().IdentUnexported("type")) // default unchanged
}

func TestWithGoFileRepairSuffix(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler(WithGoFileRepairSuffix("gen"))
	assert.EqualT(t, "config_linux_gen", g.File("config_linux"))         // custom suffix
	assert.EqualT(t, "test_swagger.go", MakeGoMangler().File("test.go")) // default unchanged
}

func TestWithGoInitialisms(t *testing.T) {
	t.Parallel()

	t.Run("adds a custom acronym on top of the defaults", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler(WithGoInitialisms("FOOBAR"))
		assert.EqualT(t, "FOOBARService", g.IdentExported("foobar service")) // custom recognized
		assert.EqualT(t, "HTTPServer", g.IdentExported("http server"))       // defaults still active
	})

	t.Run("accumulates across calls", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler(WithGoInitialisms("FOO"), WithGoInitialisms("BAR"))
		assert.EqualT(t, "FOOBARBaz", g.IdentExported("foo bar baz"))
	})
}

func TestUseGoInitialisms(t *testing.T) {
	t.Parallel()

	t.Run("replaces the defaults", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler(UseGoInitialisms("ZZZ"))
		assert.EqualT(t, "ZZZ", g.IdentExported("zzz"))   // custom recognized
		assert.EqualT(t, "Http", g.IdentExported("http")) // HTTP no longer an initialism
	})

	t.Run("no arguments keeps the defaults", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler(UseGoInitialisms())
		assert.EqualT(t, "HTTPServer", g.IdentExported("http server"))
	})

	t.Run("combines with WithGoInitialisms (replace then add)", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler(UseGoInitialisms("ZZZ"), WithGoInitialisms("QQQ"))
		assert.EqualT(t, "ZZZQQQ", g.IdentExported("zzz qqq"))
		assert.EqualT(t, "Http", g.IdentExported("http")) // default gone
	})
}

func TestPointerConstructors(t *testing.T) {
	t.Parallel()

	// New* return non-nil pointers that behave like their value forms
	assert.EqualT(t, MakeMangler().Camelize("foo bar"), NewMangler().Camelize("foo bar"))
	assert.EqualT(t, MakeGoMangler().IdentExported("foo bar"), NewGoMangler().IdentExported("foo bar"))
}

func TestMakeTargetTransformCustomSeparator(t *testing.T) {
	t.Parallel()

	target := MakeTargetTransform(WithSeparator("."))
	assert.EqualT(t, "foo.bar.baz", MakeMangler().Transform(target, "foo bar baz"))
}

func TestWithTokenSeparatorPredicate(t *testing.T) {
	t.Parallel()

	// only '.' separates; a custom predicate via WithTokenSeparator + WithTokenOptions
	m := MakeMangler(WithTokenOptions(WithTokenSeparator(func(r rune) bool { return r == '.' })))
	assert.EqualT(t, "aB", m.Camelize("a.b"))
}

// TestWithSymbolWords covers the customizable symbol set: overriding a word, adding a symbol (which changes
// segmentation — the rune becomes a token instead of a separator), and removing one (which makes it a separator). It
// also checks the built-in default set is never mutated.
func TestWithSymbolWords(t *testing.T) {
	t.Parallel()

	t.Run("override a word", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler(WithManglerOptions(WithSymbolWords(map[rune]string{'@': "arobase"})))
		assert.EqualT(t, "AArobaseB", g.ConstName("a@b"))          // custom word
		assert.EqualT(t, "AAtB", MakeGoMangler().ConstName("a@b")) // default unchanged
	})

	t.Run("add a symbol changes segmentation", func(t *testing.T) {
		t.Parallel()
		// ',' is a separator by default ("a,b" -> "AB"); adding it to the set makes it a verbalizable symbol token.
		g := MakeGoMangler(WithManglerOptions(WithSymbolWords(map[rune]string{',': "comma"})))
		assert.EqualT(t, "ACommaB", g.ConstName("a,b"))
		assert.EqualT(t, "AB", MakeGoMangler().ConstName("a,b")) // default: ',' is a separator
	})

	t.Run("remove a symbol makes it a separator", func(t *testing.T) {
		t.Parallel()
		// An empty word removes '@' from the set, so it segments as a separator (dropped) instead of verbalizing.
		g := MakeGoMangler(WithManglerOptions(WithSymbolWords(map[rune]string{'@': ""})))
		assert.EqualT(t, "AB", g.ConstName("a@b"))
	})

	t.Run("base Mangler honors it too", func(t *testing.T) {
		t.Parallel()
		m := MakeMangler(WithSymbolWords(map[rune]string{'@': "arobase"}))
		assert.EqualT(t, "A arobase b", m.Humanize("a@b")) // Humanize titleizes the first word
	})

	t.Run("DefaultSymbolWords returns an independent copy", func(t *testing.T) {
		t.Parallel()
		d := DefaultSymbolWords()
		d['@'] = "MUTATED"
		delete(d, '&')
		// mutating the copy must not leak into a freshly built mangler
		assert.EqualT(t, "AAtB", MakeGoMangler().ConstName("a@b"))
		assert.EqualT(t, "AAndB", MakeGoMangler().ConstName("a&b"))
	})
}

// TestBaseManglerNamesNumeralRune exercises expandRuneNames' numeral branch on the base Mangler path.
//
// A numeral rune is spelled out as words (unlike literal ASCII digits, which Camelize leaves alone).
func TestBaseManglerNamesNumeralRune(t *testing.T) {
	t.Parallel()

	m := MakeMangler(WithASCIIFolding(true))
	assert.EqualT(t, "oneHalfCup", m.Camelize("½ cup"))
}

// TestModuleNonVersionSuffix hits majorVersionDigits' non-digit exit.
//
// "vbeta" looks like a version but isn't, so it is not rewritten to "version…".
func TestModuleNonVersionSuffix(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler()
	assert.EqualT(t, "foo/vbeta", g.Module("foo/vbeta")) // not a version → untouched
	assert.EqualT(t, "foo/version2", g.Module("foo/v2")) // real version → rewritten
}

// TestDefaultSeparatorInjected guards the invariant that a constructed mangler always carries the full default
// separator predicate.
//
// The tokenizer's own nil-Separator fallback is a minimal whitespace/non-graphic rule; if a constructor forgot to
// inject the default, segmentation would silently switch to that weaker (and slower) rule for punctuation. This pins
// the injection for both the base Mangler and the GoMangler, checking a rune the default splits on (a comma, category
// Po) that the minimal fallback would not.
func TestDefaultSeparatorInjected(t *testing.T) {
	t.Parallel()

	for name, sep := range map[string]func(rune) bool{
		"Mangler":   MakeMangler().Separator,
		"GoMangler": MakeGoMangler().Separator,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if sep == nil {
				t.Fatal("constructed mangler has a nil Separator (default not injected)")
			}
			assert.Truef(t, sep(','), "%s: default separator must split on ',' (Po)", name)     // the full rule; basicSeparator would miss it
			assert.Truef(t, sep('-'), "%s: default separator must split on '-' (Pd)", name)     // dashes
			assert.Falsef(t, sep('@'), "%s: '@' verbalizes as a symbol, not a separator", name) // guarded by defaultSymbolWords
			assert.Falsef(t, sep('a'), "%s: letters are never separators", name)
		})
	}
}

// TestTokenizeEarlyBreak covers the yield-returns-false path (consumer stops iterating early).
func TestTokenizeEarlyBreak(t *testing.T) {
	t.Parallel()

	tk := tokens.Tokenizer{Separator: defaultTokenSeparator}
	n := 0
	for range tk.Tokenize("a b c") {
		n++

		break
	}
	assert.EqualT(t, 1, n)
}

// TestTokenizeOrphanLeadingMark covers the orphan/leading combining-mark branch in segment
// (the mark is kept in a word run rather than silently lost).
func TestTokenizeOrphanLeadingMark(t *testing.T) {
	t.Parallel()

	tk := tokens.Tokenizer{Separator: defaultTokenSeparator}
	got := slices.Collect(tk.Tokenize("́abc")) // leading combining acute
	assert.Truef(t, slices.Equal([]string{"́abc"}, got), "got %v", got)
}
