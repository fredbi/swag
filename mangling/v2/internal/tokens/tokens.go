// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package tokens is the mangler's token engine: the zero-copy token model plus the opinionated segmentation that
// produces it.
//
// It is the mechanism the root mangler builds its rules on — it holds no naming policy (no initialisms, no fold
// tables, no target recipes). Stages that carry policy live in the root package and drive this model through its
// index-based API (Len/Text/Kind/Span/Rewrite) and the Overlay compaction primitive.
//
// It is internal for now; a public custom-pipeline surface may re-export a curated part of it later.
package tokens

import (
	"github.com/go-openapi/swag/pools"
)

// Pooled backings for the zero-copy token model, reused across mangling calls.
//
// The token slice is the churny one; the rune slice holds the single shared copy of the input.
var (
	tokenSlicePool = pools.NewPoolSlice[token]()
	runeSlicePool  = pools.NewPoolSlice[rune]()
)

// Kind classifies a token produced by segmentation.
//
// The tokenizer emits [KindWord], [KindNumber] and [KindSymbol].
// [KindInitialism] is set later by a rule overlay (the initialism pass), never by the tokenizer.
type Kind uint8

const (
	KindWord       Kind = iota // a run of letters
	KindNumber                 // a run of decimal digits (Nd)
	KindSymbol                 // a single non-letter, non-digit, non-separator rune (@, #, …)
	KindInitialism             // retagged by an overlay (HTTP, JSON, …)
)

// token is a zero-copy view into the shared []rune of a [Tokens] value.
//
// A half-open span plus the classification computed by the scanner.
//
// It is unexported: callers reach token data only through [Tokens]' index-based methods, so the struct can evolve
// without touching any consumer.
type token struct {
	start, end int    // half-open span [start,end) into Tokens.runes
	kind       Kind   // word | number | symbol | initialism
	override   string // rewritten content; empty unless a stage replaced the span
}

// Tokens is the mutable, pooled token model: a slice of [token] spans over one shared []rune (the only full copy of
// the input).
//
// Pipeline stages mutate it in place; strings are materialized only at assembly.
// It is borrowed from a pool for the duration of one mangling and released with [Tokens.Redeem]; it must not be
// retained afterwards.
//
// The surface is index-based (the [token] struct stays private): a stage reads with Len/Text/Kind/Span and edits with
// Rewrite, or merges runs with Overlay.
type Tokens struct {
	runes *pools.Slice[rune]
	toks  *pools.Slice[token]

	// count is the logical number of tokens.
	//
	// It equals the pooled slice length after segmentation, but an overlay that merges tokens in place shrinks it below
	// the slice length, so only toks[:count] are live.
	count int

	releaseRunes func()
	releaseToks  func()
}

// Borrow borrows a [Tokens] and loads the input as one shared []rune.
//
// It returns the wrapper by value so it stays on the caller's stack (no heap alloc): the pooled slices and their
// redeem closures are cached by pools, so nothing here allocates.
func Borrow(in string) Tokens {
	// len(in) bytes is an upper bound on the rune count, so the pre-grown slice never reallocates.
	runeSlice, releaseRunes := runeSlicePool.BorrowWithSizeAndRedeem(len(in))
	for _, r := range in {
		runeSlice.Append(r)
	}
	tokSlice, releaseToks := tokenSlicePool.BorrowWithRedeem()

	return Tokens{
		runes:        runeSlice,
		toks:         tokSlice,
		releaseRunes: releaseRunes,
		releaseToks:  releaseToks,
	}
}

// --- read API ---

// Len is the number of live tokens.
func (t *Tokens) Len() int { return t.count }

// Text returns the content of token i: its rewritten override if set, else its rune span.
func (t *Tokens) Text(i int) string {
	tk := t.toks.Slice()[i]
	if tk.override != "" {
		return tk.override
	}

	return string(t.runes.Slice()[tk.start:tk.end])
}

// Kind returns the kind of token i.
func (t *Tokens) Kind(i int) Kind { return t.toks.Slice()[i].kind }

// Span returns token i's raw rune span (a view into the shared slice — no copy) and its override (empty unless a
// transform rewrote it).
func (t *Tokens) Span(i int) ([]rune, string) {
	tk := t.toks.Slice()[i]

	return t.runes.Slice()[tk.start:tk.end], tk.override
}

// Bounds returns token i's half-open rune-span bounds [start,end) into [Tokens.Runes].
//
// It lets an overlay inspect contiguity between adjacent tokens (a separator elided between them leaves a gap) without
// exposing the [token] struct.
func (t *Tokens) Bounds(i int) (start, end int) {
	tk := t.toks.Slice()[i]

	return tk.start, tk.end
}

// Runes returns the shared input runes backing the tokens (read-only view — do not mutate).
//
// Together with [Tokens.Bounds] it lets a rule read the raw runes of any token (e.g. case-insensitive initialism
// matching) without copying.
func (t *Tokens) Runes() []rune { return t.runes.Slice() }

// RuneLen is the number of runes in the shared input (a size hint for assembly).
func (t *Tokens) RuneLen() int { return t.runes.Len() }

// --- write API (mutating an element in place is safe; growing goes through the pool wrapper) ---.

// Rewrite replaces the rendered content of token i (transliteration, inflection, verbalization).
func (t *Tokens) Rewrite(i int, s string) {
	t.toks.Slice()[i].override = s
}

// Overlay runs a single forward-compaction pass over the tokens.
//
// At each position from, match may claim a run of span tokens [from, from+span): the run is merged into one token
// spanning their combined rune range, retagged to kind and carrying override; span == 0 leaves the token as-is. The
// merge shrinks the slice in place (matched runs never grow it).
//
// This is the mechanism behind rule overlays such as the initialism pass (which merges break-crossing runs like
// [ipv,4] into one canonical token); the *matching* policy lives with the rule, the compaction stays here.
func (t *Tokens) Overlay(match func(from int) (span int, kind Kind, override string)) {
	toks := t.toks.Slice()
	n := t.count

	w := 0
	for r := 0; r < n; {
		if span, kind, override := match(r); span > 0 {
			merged := toks[r]
			merged.end = toks[r+span-1].end
			merged.kind = kind
			merged.override = override
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

// Redeem returns the pooled backings.
//
// The tokens must not be used afterwards.
func (t *Tokens) Redeem() {
	t.releaseToks()
	t.releaseRunes()
}

// push appends a token spanning [start,end) with its classification.
//
// Scanner-only.
func (t *Tokens) push(start, end int, kind Kind) {
	t.toks.Append(token{start: start, end: end, kind: kind})
	t.count++
}
