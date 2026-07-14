// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package tokens

import (
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// spaceSep is a minimal separator for the tests (the root package injects a richer default).
func spaceSep(r rune) bool { return r == ' ' }

func TestSegment(t *testing.T) {
	t.Parallel()

	tk := Tokenizer{Separator: spaceSep}
	got := slices.Collect(tk.Tokenize("fooBar baz2 HTTPServer"))

	// lower→Upper, letter↔digit and Upper-run→lower boundaries, separators elided.
	assert.Truef(t,
		slices.Equal([]string{"foo", "Bar", "baz", "2", "HTTP", "Server"}, got),
		"got %v", got,
	)
}

func TestKindAndSpan(t *testing.T) {
	t.Parallel()

	tk := Tokenizer{Separator: spaceSep}
	tokens := Borrow("ab 12 @")
	defer tokens.Redeem()
	tk.Segment(&tokens)

	assert.EqualT(t, 3, tokens.Len())
	assert.EqualT(t, KindWord, tokens.Kind(0))
	assert.EqualT(t, KindNumber, tokens.Kind(1))
	assert.EqualT(t, KindSymbol, tokens.Kind(2))

	// Span exposes the raw runes and (absent a rewrite) an empty override; Bounds agrees with it.
	runes, override := tokens.Span(0)
	assert.EqualT(t, "ab", string(runes))
	assert.EqualT(t, "", override)
	start, end := tokens.Bounds(0)
	assert.EqualT(t, 0, start)
	assert.EqualT(t, 2, end)
	assert.EqualT(t, 'a', tokens.Runes()[start])
}

func TestRewrite(t *testing.T) {
	t.Parallel()

	tk := Tokenizer{Separator: spaceSep}
	tokens := Borrow("cafe")
	defer tokens.Redeem()
	tk.Segment(&tokens)

	tokens.Rewrite(0, "CAFÉ")
	assert.EqualT(t, "CAFÉ", tokens.Text(0))
}

func TestOverlay(t *testing.T) {
	t.Parallel()

	tk := Tokenizer{Separator: spaceSep}
	tokens := Borrow("a b c d")
	defer tokens.Redeem()
	tk.Segment(&tokens)
	assert.EqualT(t, 4, tokens.Len())

	// Merge the run [0,2) ("a","b") into one initialism token; leave the rest. The compaction shrinks the slice.
	tokens.Overlay(func(from int) (int, Kind, string) {
		if from == 0 {
			return 2, KindInitialism, "AB"
		}

		return 0, 0, ""
	})

	assert.EqualT(t, 3, tokens.Len())
	assert.EqualT(t, "AB", tokens.Text(0))
	assert.EqualT(t, KindInitialism, tokens.Kind(0))
	assert.EqualT(t, "c", tokens.Text(1))
	assert.EqualT(t, "d", tokens.Text(2))
}

func TestIsCombiningMark(t *testing.T) {
	t.Parallel()

	assert.Truef(t, IsCombiningMark('́'), "U+0301 combining acute is a mark")       // Mn
	assert.Falsef(t, IsCombiningMark('a'), "ASCII letter is not a mark")            // below the U+0300 short-circuit
	assert.Falsef(t, IsCombiningMark('é'), "precomposed é is a letter, not a mark") // U+00E9
}
