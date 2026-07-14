# Extract the token engine to `internal/tokens`

**Goal (Fred, 2026-07-08):** move the tokenizer & token-manipulation *mechanism* into an internal
package so (1) roles/responsibilities are explicit, (2) the exposed API stays small, (3) a future
custom-pipeline / tokenizer API is easy to expose.

**Decision:** Option A (mechanism only), package `mangling/v2/internal/tokens`.

## The fault line
The token *engine* has zero dependency on root policy and moves wholesale. Everything that reads a
`TargetTransform`, the symbol table, the initialism trie, or the fold tables is *policy that consumes*
the engine — it stays in root and imports `internal/tokens`.

## Moves to `internal/tokens`
- token model: `tokens`→`Tokens`, `token` (stays unexported in-pkg), `tokenKind`→`Kind`
  (`KindWord/KindNumber/KindSymbol/KindInitialism`), pools, `borrowTokens`→`Borrow`.
- read/write API (exported): `Len`, `Text`, `Kind` (was `kindOf`), `Span` (was `span`),
  `RuneLen` (was `runeLen`), `Rewrite`, `Redeem` (was `redeem`); unexported `push`.
- NEW primitives so no root stage touches private fields:
  - `Runes() []rune` — read view of the shared slice.
  - `Bounds(i) (start, end int)` — token i's rune-span bounds.
  - `Overlay(match func(from int) (span int, kind Kind, override string))` — one forward-compaction
    pass that merges a claimed run of tokens into one (the initialism overlay's mechanism).
- segmentation: `Tokenizer{Separator func(rune) bool}` with `Segment(*Tokens)` + `Tokenize`,
  `classify`, `runeClass`, `runeCase` (all unexported); `IsCombiningMark` (was `isCombiningMark`, exported).

## Stays in root (imports `internal/tokens`)
- manglers, options (`tokenOptions`/`WithTokenSeparator`/`buildTokenOptions`), `TargetTransform` + presets.
- the **default separator** (`defaultTokenSeparator`/`separatorForRune`/`asciiSeparator`) — reads
  `defaultSymbolWords`; injected into `Tokenizer.Separator` at construction.
- assembler (`assemble`/`writeCased`/`wordCasing`/`symbolPolicy`), all stages
  (`foldASCII`/`applyInitialisms`/`verbalizeLeadingNumber`/`asciifyInput`), fold tables, trie, numbers wiring.

## Wiring changes
- `Mangler` embeds `tokens.Tokenizer` (was `tokenizer`) + `options`; construction sets
  `m.Tokenizer.Separator = m.options.separator`. `Tokenize` stays public (promoted). `m.segment(&t)` → `m.Segment(&t)`.
- Call-site renames in root: `borrowTokens`→`tokens.Borrow`, `t.redeem()`→`t.Redeem()`,
  `t.kindOf(i)`→`t.Kind(i)`, `t.span(i)`→`t.Span(i)`, `t.runeLen()`→`t.RuneLen()`, `kindWord`→`tokens.KindWord` (etc.),
  `isCombiningMark`→`tokens.IsCombiningMark`.
- `initialisms.go`: `trie.match` reads via `t.Runes()`/`t.Bounds()`; `applyInitialisms` drives `t.Overlay(...)`.
  Removes the raw `[]token`/`t.toks`/`t.runes` access (fixes the model's own "index-based only" invariant, which
  the overlay currently violates).
- tests: `TestTokenizeEarlyBreak`/`TestTokenizeOrphanLeadingMark` swap
  `tokenizer{tokenOptions:…}` → `tokens.Tokenizer{Separator: defaultTokenSeparator}` (stay in root).

## Risk / verification
Hot path (`Segment`/`classify`/`Span`/`IsCombiningMark`) now crosses a package boundary — all tiny, should
still inline. **Verify no regression** with `BenchmarkGoManglerPaths` before/after (go1.26), plus `go test ./...`
and `golangci-lint run`.

## Outcome (done, 2026-07-08)
Executed as planned. Package `internal/tokens` created; root consumes it. All tests + `-race` green, lint 0.

**Latent bug the A/B benchstat surfaced.** The first fast-path measurement regressed +6–11% (p<0.01, interleaved).
Root cause was *not* inlining: `buildOptions`/`buildGoOptions` never call `buildTokenOptions`, so `options.separator`
is **nil** for a plain `MakeMangler()`/`MakeGoMangler()`. The old `segment` had `if sep == nil { sep = defaultTokenSeparator }`
— the *fast* ASCII-table predicate. The new internal `Segment`'s nil fallback (`basicSeparator`) is the *slow*
full-Unicode rule **and** subtly wrong (misses the punctuation categories). Every input had been silently relying on
the runtime fallback. Fix: default `separator` in `buildOptions` **and** `buildGoOptions` (where every other default
lives), so the constructor injects the real predicate once. Guarded by `TestDefaultSeparatorInjected` (asserts a
constructed mangler's `Separator` is non-nil and splits on `,`/`-`). After the fix the fast path is at
parity-or-better (~526ns/1 alloc) vs the pre-refactor baseline; all other paths unchanged.

Per-token accessors (`Span`/`Kind`/`Len`/`RuneLen`/`Bounds`/`Runes`/`push`) all inline cross-package, confirmed via
`-gcflags=-m`. `internal/tokens` has its own `tokens_test.go` (Segment/Kind/Span/Rewrite/Overlay/IsCombiningMark).
