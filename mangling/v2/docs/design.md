# Design

How this package is built, and where it is headed.

This is the architecture behind the three manglers and the rationale for each.

The job is narrow and old: turn arbitrary words and sentences (schema names, API identifiers,
free text) into strings a code generator can safely emit — exported and unexported identifiers,
file names, package and module paths, constant names, human-readable titles.

The design pursues four goals at once: **correctness** on the inputs v1 got wrong,
**composability** so every output is a preset over one mechanism,
**new capabilities** (ASCII folding, number verbalization, Unicode rune-naming) folded into that mechanism
rather than bolted on, and **performance** on the hot path (1 alloc/op for the ident path).

> **Background**
>
> Historically, `go-swagger` (the main code generator to produce a Go API from an OpenAPI spec) has relied
> on name mangling utilities in `github.com/go-openapi/swag` (e.g. `swag.ToGoName`), later specialized in
> `github.com/go-openapi/swag/mangling.NameMangler`. Below this is referred to as *v1* (technically, `v0.x`).
>
> This layer has been improved over time but has always suffered from a few core design limitations, that have
> surfaced in the generated outcome of `go-swagger` and could never be fixed with non-breaking changes.
>
> This new package provides a new API based on stronger principles to gradually replace the `swag/mangling`
> package that will eventually disappear from `swag`'s APIs (`swag/v2` will drop mangling among other things).

## One idea: boundaries are explicit, not a side effect of casing

Everything follows from a single modeling choice. A **case transition is a first-class word
boundary**, decided during segmentation, before anything else looks at the tokens. `fooBar` splits
at `o→B`; `HTTPServer` splits at `P→S` with a one-rune lookback so the acronym stays whole. A run of
capitals with no following lowercase is therefore *one* token — `THIS_IS_ALL_CAPS` segments to
`[THIS, IS, ALL, CAPS]`, never shattering into single letters.

This is the inversion of v1's central flaw, where the initialism dictionary doubled as the
segmenter and neither could be fixed without breaking the other. Here **initialisms are an overlay
that retags tokens; they never create boundaries.**

See [v1-differences.md](v1-differences.md) for the full before/after.

## The pipeline

```
input ──▶ segment ──▶ [ transforms ] ──▶ assemble ──▶ validate / repair ──▶ output
                       │                  (casing × separator × affix)
                       ├─ asciify         (fold diacritics, romanize runes, route numerals)
                       ├─ initialism      (recognize & retag, incl. sub-token & plural)
                       └─ verbalize       (numbers and symbols → words)
```

Each stage has a single job, and the order is opinionated and owned by the ruleset (folding runs
before initialism matching so a fold can't hide a match). Callers pick a preset;
they do not reorder the chain.

### 1. Segmentation

A run of characters becomes a token at one of these boundaries, in precedence order:

1. **Explicit separator** — `_`, `-`, space, `.` (consumed, not emitted).
2. **Case transition** — `lower→Upper`, and `Upper-run→lower` with one-rune lookback.
3. **Letter ↔ digit** — `v4 → v | 4`, `oauth2 → oauth | 2`.
4. **Script / category change** — letter ↔ symbol, letter ↔ number.

Segmentation is deliberately **dictionary-free**: it knows nothing about initialisms, which is what
keeps the two concerns from fusing. The tokenizer is opinionated — there is no public rule-injection
API in 1.0 (only `WithTokenSeparator`), because the boundary signals above are the right defaults for
Go and exposing them prematurely would freeze a contract we may still refine.

### 2. The token model — zero-copy

The input is copied into a rune slice exactly once. Every token is a *view* — a span into that
slice, plus its kind and casing pattern — so no stage allocates a per-token string:

```go
type token struct {
    start, end int         // span into the shared []rune
    kind       tokenKind   // casual | initialism | number | symbol | ...
    casing     casePattern // lower | upper | title | mixed | screaming
    override   string      // set only when a transform rewrote the content
}
```

The token slice is drawn from a `sync.Pool` and edited in place through the stages.
A string is materialized only once, when the final output is assembled.
This is the whole allocation story: one pooled token slice, one output buffer.

### 3. Transforms

Each transform is a pure function over the token stream — it captures no mutable per-call state, so
a compiled recipe runs concurrently without coordination.

- **ASCII folding** turns non-ASCII letters and symbols into ASCII, in tiers from cheapest to most
  approximate: Latin diacritics fold to their base (`café → cafe`), decimal digits map by value,
  combining marks are stripped, and everything else is romanized by its Unicode rune name
  (`ж → Zhe`, `😀 → GrinningFace`). CJK Han ideographs have no phonetic name and are elided. Full
  detail and the tier table in [asciification.md](asciification.md).

- **Initialism recognition** is an overlay on the segmented stream. It matches whole tokens
  (`HTTP`, `JSON`), longest-prefix sub-tokens inside an all-caps run (`IDS → ID + S`), and adjacent
  windows for acronyms that contain their own boundaries (`IPv4`, `UTF8`). Plurals (`IDs`, `URLs`)
  are the same window match with precomputed plural keys. Because it matches *token windows*, not
  rune substrings, the fuzz edge cases that plagued v1 (`TTLss` must not surface `TLS`) are
  structurally impossible — the token boundary is the guard. See
  [go-identifiers.md](go-identifiers.md).

- **Verbalization** turns non-word input into words: numbers (`42 → forty two`, `0.25 → one
  quarter`) via the [numbers](numbers.md) engine, and symbols/emoji via the rune-name fallback. This
  is position-aware — a leading marker (`@id`) drops to `Id`, an interior symbol verbalizes. The
  policy belongs to the target, not to a global table, which is why the same `@` no longer becomes
  `At` everywhere.

### 4. Assembly and repair

A **Target** is a compiled, immutable recipe describing how to render the stream:
`casing × separator × affix × repair`. Presets (`TargetCamel`, `TargetSnake`, …) are functions
returning fresh values; the mangler methods (`Camelize`, `Snakize`, …) are thin wrappers over
`Transform(target, s)`. There is exactly one assembly mechanism — no per-output casing logic.

After assembly, a target may run a **validity check with a repair strategy**. This is the seam where
Go-specific naming rules live:

| Target | Check | Repair |
|---|---|---|
| unexported identifier | not a Go keyword / predeclared | word suffix (`type → typeVar`) |
| exported identifier | *(never collides — keywords are all lowercase)* | none |
| file name | last `_`-segment ∉ {GOOS, GOARCH, `test`} | append a safe token (`test_swagger.go`) |
| package name | lower, no separator, not reserved dir | repair short name (`main → mainpkg`) |
| module path | valid module-path string | repair |

Repair is applied only where a collision can actually occur, for **minimum distortion**: `type`
unexported becomes `typeVar`, but `type` exported stays `Type` — a perfectly good name that needs no
mangling. The explicit `IdentExported` / `IdentUnexported` split is what earns this; every generated
symbol calls the mangler once and lands with the least distortion for its role.

Path-returning targets (`Package`, `Module`) preprocess the path themselves — split on the last `/`,
keep the VCS prefix verbatim, and hand only the basename to the core — so the tokenizer never has to
learn that `/` is path grammar rather than a word boundary.

### Rulesets bundle data with targets

A ruleset carries its dictionaries (initialisms, reserved words, repair tokens) *and* its named
targets together. Targets hold the recipe only; the dictionaries live on the mangler and bind to the
stages at run time, so the same target degrades gracefully across manglers — a `GoMangler` target
run on a bare `Mangler` simply finds no initialisms. Go is the only concrete ruleset today, but the
seam is where another language would plug in.

## Immutability and concurrency

A configured mangler is read-only. It is built once with functional options — `MakeXxx` returns a
value, `NewXxx` a pointer — and never mutates afterward (no v1-style `AddInitialisms`). Shared
dictionaries are read-only; per-call scratch comes from a pool. One instance is safe to share across
goroutines, verified under `-race` with 64 goroutines hammering all output methods against golden
values.

## Determinism and the identifier guarantee

Same input plus same ruleset always yields the same output. The `GoMangler` carries a stronger,
fuzz-proven contract: for **any** input, `IdentExported` / `IdentUnexported` / `ConstName` / `File`
produce a valid, non-empty Go identifier (`go/token.IsIdentifier`) with correct export visibility,
in both folding modes. An input that reduces to nothing yields a cased fallback word rather than the
empty string. The fuzz suite converged clean over 1.4M executions and surfaced seven real contract
bugs, all fixed and kept as regression seeds.

## Performance

The ident hot path is **1 alloc/op** — one pooled token slice (zero-copy rune views) plus one
assembly buffer — and scales linearly at roughly 380 ns/token, flat from 1 to 1024 tokens. The
number verbalizer streams into a single byte sink instead of building `[]string` + `join`:
`NumberWords` is 0 alloc for non-numeric input and 1 for numeric; the allocation-free `AppendWords`
variant is 0 when the caller pools the destination. See [performance.md](performance.md).

> **Performance background**
>
> Mangling functions may be called in the 10-thousands by the current code generator.
> We want mangling to avoid putting any more pressure than necessary on Go's GC and keep a low profile
> on peak allocated memory. So even if the gc is fast at cleaning up garbage, we prefer *not creating*
> garbage objects at all.
>
> The new design proved successful with that approach, with a modest but real 30% performance improvement
> in timings (with much more work being carried out for correctness),
> which indicates that even with Go 1.26, pooling can still benefit significantly.

## Roadmap

The core is complete and the public API is frozen for 1.0. The shape below is settled.
Other features are mostly deferred to forthcoming minor releases, so the surface can freeze without waiting on them.

**Near term**

- **Unicode v17.** Lands with go1.27. A second generated table set guarded by `//go:build go1.27`,
  selected against v15 at compile time so only one links. Additive and prepared ahead of time; it
  does not gate 1.0.

**Later — additive, does not touch the core encoding**

- **A `plurals` package.** The ~200 lines of English inflection actually used (regular rules plus
  irregular/uncountable tables), extracted as standalone `Pluralize` / `Singularize` functions, so
  `go-openapi/inflect` can finally be archived. The narrow use case is doc-comment generation
  (`MyArray is a collection of MyTypes`); initialism plurals would share the engine.

- **Grapheme clusters.** A pre-pass that groups codepoints before rune-naming, so multi-codepoint
  units resolve as one name: flag sequences (`🇮🇪` → ISO-3166 → `Ireland`), ZWJ emoji (family
  sequences), skin-tone and variation-selector modifiers. Purely additive — it sits in front of the
  existing per-codepoint path. Value is thin for real identifiers, so it is a showcase, not a
  priority. Single-codepoint emoji work today.

- **CJK & Hangul support.** Two distinct scripts are elided today, for different reasons. Japanese
  *kana* already romanize (`こんにちは → KoNNiTiHa`); the gap is the shared CJK Unified Ideographs block
  (Kanji / Hanzi), currently elided to a valid fallback — romanizing Han needs a word-keyed source
  (CEDICT) because character-level pinyin is unreliable for polyphonic characters, and a separate
  build-tagged sub-table to stay off the default budget. **Korean Hangul** is also elided: the composed
  syllables (`가`–`힣`) are algorithmic like Han, and the standalone **Jamo** letters (`ㄱ`, `ㅏ`) are
  currently dropped rather than romanized — a cheap future improvement would name them by their letter
  name (`ㄱ → kiyeok`, `ㅏ → a`), consistent with Greek/Cyrillic, but standalone Jamo are rare in
  identifiers. The architecture stays open to both; whether either clears the value bar is uncertain.

- **A public token-injection API.** The token model is intentionally unexported for 1.0 — it was
  exported but had no injection surface, so nothing could use it. A deliberate `InjectStage` hook
  over the token model can be re-exposed post-1.0 without breaking the frozen API, turning the
  tokenizer into a genuinely composable primitive for callers who need a custom stage.

- **Cross-script segmentation boundary.** Segmentation splits letter↔symbol and letter↔digit today,
  but not letter↔letter across scripts (`café日本` stays one token). It is near-moot while folding is
  on (CJK is elided before segmentation) and only matters for the folding-off base `Mangler` on
  genuinely mixed-script input — deferred until a concrete need appears.

v2 is expected to graduate into its own repository as a core codegen primitive shared across the
go-openapi / go-swagger ecosystem. v1 remains maintained and frozen; there are no
backward-compatibility constraints between the two.
