# Mangling v2 — design

> Status: **draft / in discussion** (started 2026-06-05, branch `exp/mangling-v2`).
> This document is exploratory. v2 is expected to graduate into its own repository as a core,
> reusable codegen primitive shared across the go-openapi / go-swagger ecosystem. v1 (`go-openapi/swag/mangling`)
> remains maintained and frozen for a long time. There are **no backward-compatibility constraints** on the v2 API.

## 1. Motivation & goals

Name mangling turns arbitrary words and sentences (schema names, API identifiers, free text) into strings that a
code generator can safely emit: exported/unexported identifiers, file names, package names, JSON keys, doc comments.

v2 pursues four drivers at once:

1. **Correctness** — fix the documented v1 limitations, all of which trace back to a single modeling flaw (see §2).
2. **Composability** — replace the bespoke `ToXXX` methods with a composable model (*segment → transform →
   assemble → validate*) and express the familiar outputs as presets over it.
3. **New capabilities by consolidation** — absorb Go naming know-how currently dispersed in go-swagger (reserved
   words, reserved file-name suffixes, reserved dir names, package/module/file naming) and fuse inflection
   (singular/plural), retiring `go-openapi/inflect`.
4. **Performance** — keep (ideally beat) v1's ~3 allocs/op for `ToGoName`; never regress on the hot path.

### Non-goals (for this iteration)

- **Not language-agnostic from day one.** v2 is **Go-centric but pluggable**: the Go ruleset is first-class, and the
  core is structured so other-language rulesets can slot in later. We do not pay the full generalization tax now.
- **Not an environment resolver.** Computing *which* module/package you are in (`go.mod` parsing, GOPATH walking,
  base-import resolution) is build-context resolution, not string mangling. It stays out of this package (a shared
  "go environment" helper may host it later). v2 only decides whether a *name string* is legal and how to repair it.

## 2. Background: v1 and why it breaks

v1 pipeline: `input → split into initialism-aware lexemes → reassemble with per-target casing + separator`.

It works well and is fast (3 allocs/op after PR #106), but every documented bug has **one root cause**:
**case alternance was never modeled as a word boundary.** Lacking it, camelCase still had to be split somehow, so the
*initialism matcher* became the de-facto segmenter — segmentation and initialism recognition got fused (originally for
allocation reasons, the two passes each allocated heavily). Once initialisms carried segmentation, neither could be
fixed without breaking the other. That is the "initialism trap."

Symptoms, all the same disease:

| Symptom | Example | Cause |
|---|---|---|
| All-caps explodes | `ToFileName("THIS_IS_ALL_CAPS")` → `t_h_i_s_i_s_a_l_l_c_a_p_s` | "uppercase rune = boundary" instead of "case *transition* = boundary"; a run of caps is shattered |
| Fragile initialism boundaries | `IDS` vs `IDx` vs `IDs` produce inconsistent splits | global rune-scan lookahead heuristics entangled with matching |
| English-only inflection | only `+s`, terms ending in `s` forced invariant | pluralization hardcoded into the matcher |
| Linter-unsafe output | unicode identifiers trip `asciicheck`/`gosmopolitan` | no transliteration/folding stage |
| Blind verbalization of values | `@type`/`@id` → `AtType`/`AtId`; `12` → unusable; emoji → dropped | one global rune→word table, position-blind; values forced through `ToGoName` |
| Bespoke targets | each `ToXXX` is a hand-written method; two splitter instances; duplicated casing logic | no composable assembly model |
| Muddled ownership | value type holding a pointer index; `AddInitialisms` mutates shared state, not concurrency-safe | construction vs mutation not separated |

## 3. Design principles

- **Names and values are distinct input classes.** Word-like *names* are segmented; arbitrary *values* (numbers,
  symbols, emoji → `const` names) are *verbalized*. v1's quirks stem from forcing both through one identifier path.
- **Self-driven for names, knob-driven for values.** The asymmetry is one of *ownership*: name targets assume
  more-or-less acceptable input and the system owns the policy (strong defaults, little tuning). Value targets accept
  gibberish, promise only best-effort, and put the knobs in the caller's hands. The API shape reflects this — name
  methods take a string; value methods take a string plus explicit policy options.
- **Segment once, assemble many ways.** Segmentation is independent of, and prior to, every other concern.
- **Boundaries are explicit signals, not a side effect of casing.** Case alternance is a first-class boundary.
- **Initialisms are an overlay, never a segmenter.** They recognize/retag tokens; they never *create* boundaries.
  This is the clean inversion of the v1 trap.
- **Everything between segmentation and assembly is a composable transform** (a closure over the token stream).
  Initialism recognition, transliteration, folding, and inflection are all transforms.
- **Zero-copy until assembly.** Tokens are views into one shared rune slice; transforms mutate a pooled token slice
  in place; strings are materialized only when the final output is built.
- **Rulesets bundle data + targets.** A ruleset (e.g. Go) carries its dictionaries and its named targets together.
- **Immutable after construction.** A configured mangler is read-only and safe for concurrent use; no `AddInitialisms`
  mutation surface.
- **Deterministic & fuzzable.** Same input + same ruleset → same output, always.

## 4. Core model

### 4.1 Pipeline

```
input ──▶ segment ──▶ [ transforms ] ──▶ assemble ──▶ validate / repair ──▶ output
                       │                  (casing × separator × affix)
                       ├─ transliterate   (rune → ascii word, e.g. @→At, é→e)
                       ├─ fold            (normalize, strip combining marks)
                       ├─ initialism      (recognize & retag, incl. sub-token & plural)
                       └─ inflect         (singularize / pluralize)
```

The transform **order is opinionated and owned by the ruleset** (transliterate before initialism so folding can't
hide a match; initialism before inflect so plural detection sees the dictionary). Callers may **inject** transforms at
named stages, not reorder the whole chain.

### 4.2 Segmentation — boundary signals

A *run* of characters becomes a token at any of these boundaries, evaluated with precedence:

1. **Explicit separator**: `_`, `-`, space, `.`, etc. (consumed, not emitted).
2. **Case alternance**:
   - lower → Upper (`fooBar` → `foo | Bar`)
   - Upper-run → lower with one-rune lookback (`HTTPServer` → `HTTP | Server`)
3. **Letter ↔ digit** (`v4` → `v | 4`, `oauth2` → `oauth | 2`) — configurable, since some targets keep `utf8` whole.
4. **Script / Unicode category change** (Latin ↔ Han, letter ↔ symbol).

Consequence: a run of caps with no following lowercase is **one token** (`ALLCAPS` stays whole), so the all-caps
explosion is *structurally impossible*. `THIS_IS_ALL_CAPS` → `[THIS, IS, ALL, CAPS]`; none are initialisms; `FileName`
→ `this_is_all_caps`.

### 4.3 Token model (zero-copy view)

```go
type token struct {
    start, end int         // span into the shared []rune (the only full copy of the input)
    kind       tokenKind   // casual | initialism | number | symbol | ...
    casing     casePattern // lower | upper | title | mixed | screaming
    override   string      // empty unless a transform rewrote content (transliteration/inflection)
}
```

A `Tokens` value is a **pooled `[]token`** over the shared rune slice, with in-place editing operations:
`Len`, `At`, `Split`, `Merge`, `Retag`, `Rewrite`, `Range`. No string is allocated per stage; only token-slice churn
on pooled backing arrays.

### 4.4 Initialism recognition as a transform

Operates on the already-segmented token stream:

- whole-token match: retag `HTTP`, `JSON`… as initialism, preserving the dictionary's canonical casing.
- **sub-token match within an all-caps token**: `IDS` → longest dictionary prefix `ID` + remainder `S`, or plural
  `IDs`. The fragile global lookahead becomes a **local decision on a single token**, testable in isolation and
  unable to corrupt neighbors.
- plural detection shares the inflection engine (§6), so `IDs → ID` is no longer bespoke.

### 4.5 Assembly — `casing × separator × affix`

A **Target** describes how to render the token stream:

- **word casing** as a function of position (`first` may differ: `camelCase` lowercases word 0; `JSONName` too).
- **initialism casing** preserved from the dictionary regardless of position (except a leading initialism in an
  unexported identifier, which lowercases: `httpServer`).
- **separator**: `""`, `"_"`, `"-"`, `" "`, `"."`.
- **affix / prefix safeguard**: leading-non-letter repair for identifiers (v1's `PrefixFunc`, default `X`).

**Exported and unexported identifiers are the *same* target**, differing only by the initial-rune case rule. Every
other decision — segmentation, transforms, and especially the reserved-word repair (§4.6) — is shared, so the two
outputs stay pure case-variants of each other. Because all Go keywords and predeclared identifiers are lowercase, the
exported form never collides on its own; deciding repair per-target independently would desync them (`Unexported("type")`
→ `type_` while `Exported("type")` → `Type`). We therefore decide repair on the shared, case-insensitive basis and apply
it to both: `type_` / `Type_`. Symmetric repair is the **default**; an option may decouple it for callers who prefer the
exported form left un-repaired.

### 4.6 Validate / repair (the consolidation seam)

After assembly, a target may run a **validity check with a repair strategy** — this is where go-swagger's bolted-on
rules move in. Each is *detect collision → mutate*:

| Target | Check | Default repair |
|---|---|---|
| identifier | not a Go keyword/predeclared; valid identifier | trailing underscore (`type_`) |
| package name | lower, no separator, valid, not a keyword, not reserved dir | repair |
| dir name | not `vendor` / `internal` | suffix (go-swagger: `_swagger`) |
| file name | last `_`-segment ∉ {GOOS, GOARCH, `test`} | append safe token (go-swagger: `swagger`) |
| module name | valid module path string | repair |

The repair token is a **ruleset field, not a hardcode** — a neutral core cannot assume the word "swagger".

### 4.7 Verbalizing non-word input (values → identifiers)

A distinct, first-class concern that v1 mishandles: building legit identifiers — especially `const` names for enum
values — from inputs that are **not words**: numbers, symbols (`@`, `#`, `$`), emoji, mixed literals. v1 forces these
through the same `ToGoName` path with a single rune→word `replaceFunc` that is blind to position and context: every
`@` becomes `At`, so the common JSON-LD pattern `@type` / `@id` yields `AtType` / `AtId` everywhere.

v2 treats verbalization as a **position-aware, layered transform** over the segmented token stream (symbols and
numbers are already their own tokens after §4.2):

1. **Symbol policy** — `(symbol, position, target) → action ∈ {drop, verbalize, phonetic}`. A *leading marker* (`@id`)
   defaults to **drop** → `Id`/`ID`; an *interior* symbol (`read@write`) verbalizes → `ReadAtWrite`. Per-symbol and
   per-target overridable.
2. **Number verbalization** — `1 → One`, `12 → Twelve`, optionally as the **leading-digit repair** strategy (an
   alternative to the `X` prefix: `12foo → TwelveFoo` rather than `X12Foo`). Bounded and locale-aware; policy may
   instead **keep digits** for value-like targets (an HTTP status enum wants `Status200`, not `TwoHundred`).
3. **Rune-name / phonetic fallback** — for tokens with no transliteration (emoji, exotic scripts), derive a word from
   the Unicode name (`golang.org/x/text/unicode/runenames`, e.g. 😀 → "grinning face" → `GrinningFace`). Opt-in, since
   it pulls a sizable generated table.
4. **Empty-result safeguard** — if policy would yield an empty identifier (input was a lone dropped marker), fall back
   up the layers (verbalize → phonetic) so we never emit `""`.

Verbalization policy is therefore a property of the **target**, not a global table: a `const`-name target verbalizes
aggressively, a field-name target drops leading markers, a doc-comment target may keep symbols literally. This is the
seam where v1's "everything goes through `ToGoName`" pressure is relieved.

### 4.8 Ruleset = data + targets

```
GoRuleset = {
    data:    initialisms, reservedWords, reservedSuffixes, reservedDirs, inflectionRules, repairToken
    targets: GoExported, GoUnexported, PackageName, ModuleName, FileName, DirName, JSONName, DocComment
}
```

The ruleset is the seam where another language would plug in. Go is the only concrete ruleset in this iteration.

## 5. Public API sketch

> Illustrative, not final. Names and shapes are up for debate.

```go
package mangling // v2

// Mangler is an immutable, concurrency-safe engine.
type Mangler struct { /* ruleset + precomputed dictionaries + pools */ }

func New(opts ...Option) *Mangler

// Composable entry point (generic core, ruleset-neutral).
func (m *Mangler) To(target Target, input string) string

// Language facade: a ruleset bound to a mangler, exposing language-named methods.
// Keeps the generic core clean while giving Go callers ergonomic, well-named entry points.
func (m *Mangler) Go() *GoNameMangler

// --- Name targets: word-like input, self-driven (strong defaults, no per-call tuning) ---
func (g *GoNameMangler) Exported(s string) string   // was ToGoName; exported identifier
func (g *GoNameMangler) Unexported(s string) string // was ToVarName; same rules, lower initial rune
func (g *GoNameMangler) PackageName(s string) string
func (g *GoNameMangler) ModuleName(s string) string
func (g *GoNameMangler) FileName(s string) string
func (g *GoNameMangler) DirName(s string) string
func (g *GoNameMangler) JSONName(s string) string
func (g *GoNameMangler) HumanName(s string, title bool) string

// --- Value targets: arbitrary literals → const/var idents, knob-driven (best-effort; caller owns policy) ---
func (g *GoNameMangler) ConstName(value string, opts ...ValueOption) string
func (g *GoNameMangler) EnumName(typeName, value string, opts ...ValueOption) string

// Value knobs — surfaced per call because, for gibberish input, the caller (not the system) owns the policy:
func OnSymbol(p SymbolPolicy) ValueOption     // drop | verbalize | phonetic, per symbol & position
func OnNumber(p NumberPolicy) ValueOption     // to-words | keep-digits | bounded
func OnUnknownRune(p RunePolicy) ValueOption  // drop | rune-name | prefix
func WithValuePrefix(word string) ValueOption // leading-digit / empty-result safeguard

// --- Inflection (shared engine) ---
func (g *GoNameMangler) Pluralize(s string) string
func (g *GoNameMangler) Singularize(s string) string

// Exported and Unexported are the same target with the initial-case rule flipped; reserved-word
// repair is shared so they remain pure case-variants (type_ / Type_). See §4.5–4.6.

// Options
func WithRuleset(r Ruleset) Option
func WithInitialisms(words ...string) Option
func WithAdditionalInitialisms(words ...string) Option
func WithReservedWords(words ...string) Option
func WithTransliterator(t Transliterator) Option
func WithInflection(rules InflectionRules) Option
func WithRepairToken(tok string) Option
func InjectTransform(at Stage, t Transform) Option

// Transform model
type Transform func(*Tokens)
type Stage int // AfterSegment, AfterTransliterate, AfterInitialism, BeforeAssemble
```

## 6. Capabilities

> _To expand._ Per capability: motivation, plug shape, default, interaction with the pipeline.

- **Verbalization stack** (§4.7) — the layered, position-aware replacement for v1's flat `replaceFunc`:
  - *symbol policy*: per-`(symbol, position, target)` drop / verbalize / phonetic (fixes the `@id` quirk).
  - *number-to-words*: bounded, locale-aware cardinal verbalizer (`12`→`Twelve`); doubles as leading-digit repair;
    per-target "keep digits" escape (`Status200`).
  - *rune-name fallback*: opt-in `golang.org/x/text/unicode/runenames` for emoji/exotic runes.
- **ASCII folding** — diacritic folding (`é`→`e`) for linter-clean output; default table, overridable.
- **Inflection** — singular/plural engine absorbed from `go-openapi/inflect` (irregulars, uncountables, suffix
  rules). Shares the dictionary with initialism plural detection. Primary new use case: doc-comment generation.
  Closing `go-openapi/inflect` is in scope.
- **Locale-aware casing** — Turkish `i`, etc. Likely opt-in; default uses `unicode` simple casing.
- **Stopwords / abbreviation maps** — optional transforms (deferred unless needed).

## 7. Performance plan

> _To expand._ Matching algorithm (trie / Aho-Corasick vs current state machine); pooling strategy for `Tokens`;
> the alloc budget vs v1; benchmark continuity (carry the `BenchmarkToXXXName` suite forward as a regression gate).

## 8. Migration / v1 mapping

> _To expand._ Table mapping each v1 method + option to its v2 expression; note where output intentionally changes
> (all-caps, initialism boundaries) so downstream golden files are updated deliberately.

## 9. Open questions

- `override` field vs side arena for rewritten content (transliteration/inflection allocation strategy).
- ~~Reserved-word repair strategy for identifiers~~ — **resolved (§4.5–4.6)**: trailing underscore by default,
  decided on the shared case-insensitive basis so exported/unexported stay aligned (`type_` / `Type_`); symmetry is
  default, decoupling is an option. Still open: is trailing `_` the right token, or should the ruleset prefer a word
  suffix for some targets?
- Module-name rule: how much of "valid module path" belongs here vs. the future environment helper.
- Letter↔digit boundary default per target (`utf8`/`oauth2` cases).
- Facade shape: `m.Go().Exported(...)` per-call vs. constructing a `*GoNameMangler` once and reusing it.
- Number verbalization bounds & locale: cap (e.g. ≤ 9999?) then fall back to digits + prefix? English-only first?
- Leading-marker default: is *drop* always right for `@`/`#`, or per-symbol (drop `@`, but `#`→`Hash`)?
- `runenames` dependency: opt-in transform only, to keep the core table-free — confirm it never lands in the default Go ruleset.

---

## Appendix: design intent (raw notes)

> Slot reserved for Fred's intent write-up (the lost text). Paste here; I'll reconcile it into the sections above.
