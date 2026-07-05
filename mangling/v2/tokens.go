package mangling

import (
	"iter"

	"github.com/go-openapi/swag/pools"
)

// Pooled backings for the zero-copy token model (§4.3), reused across [Mangler.Transform] calls.
// The token slice is the churny one; the rune slice holds the single shared copy of the input.
var (
	tokenSlicePool = pools.NewPoolSlice[token]()
	runeSlicePool  = pools.NewPoolSlice[rune]()
)

// Tokens is the mutable, pooled token model: a slice of [token] spans over one shared []rune (the
// only full copy of the input). Transforms mutate it in place; strings are materialized only at
// assembly.
//
// A Tokens is borrowed from a pool for the duration of one mangling and released with
// [Tokens.redeem]; it must not be retained afterwards. It is the value handed to a [Transform].
//
// The public surface is deliberately **index-based** (the [token] struct stays internal): a
// transform reads with [Tokens.Len]/[Tokens.Text]/[Tokens.Kind]/[Tokens.Casing] and edits with
// [Tokens.SetKind]/[Tokens.Rewrite]/[Tokens.Split]/[Tokens.Merge].
type Tokens struct {
	runes *pools.Slice[rune]
	toks  *pools.Slice[token]

	releaseRunes func()
	releaseToks  func()
}

// borrowTokens borrows a Tokens and loads the input as one shared []rune.
//
// It returns the wrapper by value so it stays on the caller's stack (no heap alloc): the pooled
// slices and their redeem closures are cached by pools, so nothing here allocates.
func borrowTokens(in string) Tokens {
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

// runeLen is the number of runes in the shared input (a size hint for assembly).
func (t *Tokens) runeLen() int { return t.runes.Len() }

// span returns token i's raw rune span (a view into the shared slice — no copy) and its override
// (empty unless a transform rewrote it).
func (t *Tokens) span(i int) ([]rune, string) {
	tk := t.toks.Slice()[i]

	return t.runes.Slice()[tk.start:tk.end], tk.override
}

// redeem returns the pooled backings. The Tokens must not be used afterwards.
func (t *Tokens) redeem() {
	t.releaseToks()
	t.releaseRunes()
}

// push appends a token spanning [start,end) with its classification. Scanner-only.
func (t *Tokens) push(start, end int, kind Kind, casing Casing) {
	t.toks.Append(token{start: start, end: end, kind: kind, casing: casing})
}

// --- read API ---

// Len is the number of tokens.
func (t *Tokens) Len() int { return t.toks.Len() }

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

// Casing returns the case pattern of token i.
func (t *Tokens) Casing(i int) Casing { return t.toks.Slice()[i].casing }

// All ranges over the tokens' rendered text by index (read-only).
func (t *Tokens) All() iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		for i := range t.Len() {
			if !yield(i, t.Text(i)) {
				return
			}
		}
	}
}

// --- write API (mutating an element in place is safe; growing goes through the pool wrapper) ---

// SetKind retags token i — e.g. the initialism overlay marks a token [KindInitialism].
func (t *Tokens) SetKind(i int, kind Kind) {
	t.toks.Slice()[i].kind = kind
}

// Rewrite replaces the rendered content of token i (transliteration, inflection, verbalization).
func (t *Tokens) Rewrite(i int, s string) {
	t.toks.Slice()[i].override = s
}

// Split divides token i at offset at (relative to the token's start) into two adjacent tokens.
//
// TODO(#3): insert a token, adjust spans and recompute casing. Needed by sub-token initialism
// matching (IDS → ID + S).
func (t *Tokens) Split(i, at int) {
	_, _ = i, at
}

// Merge folds tokens [i, j] into a single token.
//
// TODO(#3): coalesce spans, recompute casing, drop the merged entries. Needed by the multi-token
// initialism merge across natural breaks (IPv4, UTF8).
func (t *Tokens) Merge(i, j int) {
	_, _ = i, j
}
