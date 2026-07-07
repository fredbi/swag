package mangling

import (
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

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

func TestWithSeparators(t *testing.T) {
	t.Parallel()

	tk := Tokenizer{tokenOptions: buildTokenOptions(tokenOptions{}, []TokenOption{WithSeparators('|', '/')})}
	got := slices.Collect(tk.Tokenize("a|b/c"))
	assert.Truef(t, slices.Equal([]string{"a", "b", "c"}, got), "Tokenize(a|b/c) = %v", got)
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

// TestBaseManglerNamesNumeralRune exercises expandRuneNames' numeral branch on the base Mangler path:
// a numeral rune is spelled out as words (unlike literal ASCII digits, which Camelize leaves alone).
func TestBaseManglerNamesNumeralRune(t *testing.T) {
	t.Parallel()

	m := MakeMangler(WithASCIIFolding(true))
	assert.EqualT(t, "oneHalfCup", m.Camelize("½ cup"))
}

// TestModuleNonVersionSuffix hits majorVersionDigits' non-digit exit: "vbeta" looks like a version but
// isn't, so it is not rewritten to "version…".
func TestModuleNonVersionSuffix(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler()
	assert.EqualT(t, "foo/vbeta", g.Module("foo/vbeta")) // not a version → untouched
	assert.EqualT(t, "foo/version2", g.Module("foo/v2")) // real version → rewritten
}

// TestTokenizeEarlyBreak covers the yield-returns-false path (consumer stops iterating early).
func TestTokenizeEarlyBreak(t *testing.T) {
	t.Parallel()

	tk := Tokenizer{tokenOptions: buildTokenOptions(tokenOptions{}, nil)}
	n := 0
	for range tk.Tokenize("a b c") {
		n++

		break
	}
	assert.EqualT(t, 1, n)
}

// TestTokenizeOrphanLeadingMark covers the orphan/leading combining-mark branch in segment (the mark
// is kept in a word run rather than silently lost).
func TestTokenizeOrphanLeadingMark(t *testing.T) {
	t.Parallel()

	tk := Tokenizer{tokenOptions: buildTokenOptions(tokenOptions{}, nil)}
	got := slices.Collect(tk.Tokenize("́abc")) // leading combining acute
	assert.Truef(t, slices.Equal([]string{"́abc"}, got), "got %v", got)
}
