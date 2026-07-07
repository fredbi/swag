# Mangling v2 — design

> Status: **core implemented, iterating** (started 2026-06-05, branch `exp/mangling-v2`; status refreshed 2026-07-06).
> The pipeline, the Go ruleset (idents / package / module / file / const), the initialism overlay, ASCII folding and
> the `numbers` subpackage are built and tested. Remaining work is scoped in **§11 Implementation status** — chiefly
> Unicode rune-naming and the inflection engine.
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
- **multi-token merge across natural breaks**: some initialisms *contain their own segmentation boundaries* — `IPv4`
  splits on case-alternance (`[IP, v, 4]`), `UTF8` on the letter↔digit boundary (`[UTF, 8]`). Segmentation stays
  dictionary-free (no v1 trap), so the overlay reassembles these by matching the dictionary against a **window of
  adjacent tokens**: it concatenates the window, lowercases it, and looks it up in a precomputed table (longest window
  wins). On a hit it merges + retags to the canonical casing. To keep this off the hot path, the entries that contain a
  natural break are **precomputed at `Mangler` construction** — the merge pass only runs when such entries exist
  (default: `IPv4`, `IPv6`, `UTF8`, plus every pluralized initialism; callers may add more).
- **pluralized initialisms** (`IDs`, `URLs`, `IPs`, `SiteURLs`) are the *same* window-merge, with the table extended by
  precomputed plural keys — not a bespoke path. This is the v1 pain point (`split.go`/`initialism_index.go`) redone
  cleanly:
  - *Plural precompute* (carried over from v1's `pluralForm`, at construction): an initialism is **invariant** (no
    plural key) if it ends in `S`/`s` (`DNS`, `CSS`), or if `key+"s"`/`key+"S"` is itself an initialism (the `HTTP`
    vs `HTTPS` conflict — keeps `"https"` mapping to `HTTPS`, not a spurious plural of `HTTP`); otherwise **simple**,
    adding a lowercased key → canonical `"URLs"` (base casing + **lowercase** suffix). Irregulars via
    `WithGoInitialismPlurals`.
  - *The fuzz edge (`TTLss` vs `TLS`, issue #159) is handled structurally*, because the overlay matches whole
    **token windows**, not rune-level substrings: `TTLss` segments to `[TT, Lss]`, whose windows (`tt`, `ttlss`, `lss`)
    match no key, so it stays `TTLss` — `TLS` can't be found mid-run. And v1's runtime "trailing-lowercase = new word"
    lookahead guard is free: a plural `s` followed by more lowercase lands in the *same* token
    (`IDsomething → [I, Dsomething]`, window `idsomething` ≠ `ids`), so only genuine plurals (`IDs → [I, Ds]`,
    window `ids`) match. The token boundary *is* the guard.
  - Plural generation shares the inflection engine (§6), so `URL → URLs` is not bespoke.

### 4.5 Assembly — `casing × separator × affix`

A **Target** describes how to render the token stream:

- **word casing** as a function of position (`first` may differ: `camelCase` lowercases word 0; `JSONName` too).
- **initialism casing** preserved from the dictionary regardless of position (except a leading initialism in an
  unexported identifier, which lowercases: `httpServer`).
- **separator**: `""`, `"_"`, `"-"`, `" "`, `"."`.
- **affix / prefix safeguard**: leading-non-letter repair for identifiers (v1's `PrefixFunc`, default `X`).

**Exported and unexported identifiers share the same target**, differing only by the initial-rune case rule (plus the
leading-initialism lowercasing for unexported, §4.4). One place they *diverge*: **reserved-word repair (§4.6)**.

**Repair is applied only where a collision actually occurs — the unexported form** (`Unexported("type")` → `typeVar`).
Because all Go keywords and predeclared identifiers are lowercase, a Title-cased exported name can never equal one, so
**the exported form is left undistorted** (`Exported("type")` → `Type`, `Exported("append")` → `Append`).

> Rationale (revised — was "symmetric repair" in an earlier draft): the goal is **minimum distortion of the origin
> token**. We do *not* suffix the exported form just to keep it a case-variant of the unexported one (`TypeVar`) — `Type`
> is a perfectly good exported name and needs no mangling. The explicit `IdentExported` / `IdentUnexported` API is what
> earns this: unlike v1, where everything was a "go name" and callers nested `ToVarName(ToGoName(...))` unsure which
> rule applied, every generated item (exported field, func, or a local/unexported var) now calls the mangler exactly
> once, and lands with the least distortion for its role.

The repair **token** is a ruleset field (§4.6). The Go ruleset defaults to the word suffix `Var` (matching
go-swagger's convention: `type` → `typeVar`), configurable per ruleset.

### 4.6 Validate / repair (the consolidation seam)

After assembly, a target may run a **validity check with a repair strategy** — this is where go-swagger's bolted-on
rules move in. Each is *detect collision → mutate*:

| Target | Check | Default repair |
|---|---|---|
| identifier (unexported only) | not a Go keyword/predeclared or builtin (all lowercase → only unexported can collide) | word suffix (`type`→`typeVar`) |
| package name | lower, no separator, valid, not a keyword, not reserved dir | repair |
| dir name | not `vendor` / `internal` | suffix (go-swagger: `_swagger`) |
| file name | last `_`-segment ∉ {GOOS, GOARCH, `test`} | append safe token (go-swagger: `swagger` → `test_swagger.go`) |
| module name | valid module path string | repair |

Repair is **rule-based**: *reserved-word detection (a `map[string]struct{}` set) + a single repair-token rule*.
The repair token is a **ruleset field, not a hardcode** — a neutral core cannot assume the word "swagger" (the Go
ruleset defaults idents to `Var`; app layers like go-swagger override file/dir tokens to `swagger`). A more powerful
**per-word repair map** (`map[string]string`, e.g. `type`→`typ`) is deliberately **deferred**; if needed it layers on
top of detection as an exceptions table.

#### 4.6.1 Path-returning targets (`Package`, `Module`) — preprocess, keep the tokenizer path-agnostic

Methods that produce **paths** (`Package` → `(alias, pkg)`, `Module`) must preserve `/`, which is *path grammar*, not a
word boundary. We do **not** teach the tokenizer about `/` (no "keep this separator" mode — that's exactly the
target-specific leakage §4.2 avoids). Instead these methods **preprocess the path** and delegate only word-like
fragments to the core mangler:

1. **Split on the last `/`.** The **prefix** (`github.com/toktok`) is a real VCS location — kept **verbatim**, never
   re-mangled (rewriting it would break the import). Host dots, a trailing `@version`, etc. live in the untouched
   prefix and are therefore not our concern.
2. **The basename yields two forms, by different rules:**
   - **import path** ← basename as a **path-legal element**: hyphens *kept* (legal in a path element), symbols
     verbalized, lowercased. `@alpha-beta` → `at-alpha-beta`, so `pkg = github.com/toktok/at-alpha-beta`.
   - **package name / alias** ← the **last `-`-delimited segment** of the basename, made a valid lowercase identifier.
     `alpha-beta` → `beta`; `go-redis` → `redis`. This is a *code-generation* convention (we own the emitted
     `package X` name), distinct from `IdentExported`/`Unexported`, which keep every word (`alphaBeta`).
3. **Go-toolchain short-name repairs** (on the last `-`-segment): a reserved name gets `pkg` appended directly
   (`main`→`mainpkg`; likewise `vendor`/`internal`/`testdata`); a bare major-version element `^[vV]\d+$` becomes
   `version<N>` (`xxxx/v2`→`version2`, `pkg = xxxx/version2`). The repair reflects into `pkg` and `parts` too.
   *(This supersedes an earlier note that a `/vN` should be skipped to name from the parent — go-swagger transforms
   `v2`→`version2` instead, which is simpler and needs no cross-element lookback.)*
4. **Bare input** (no `/`): basename = whole string, empty prefix; falls out naturally.

Implemented as `Package(pth) (shortName, pkg)` and `PackageWithParts(pth) (shortName, pkg, parts)` — the latter hands
the caller the `[x, y, z]` parts to make its own alias-deconfliction decisions. Only `/` is a separator (a package path
is not a filesystem path); a trailing `/` is trimmed first. NOT yet applied: reserved-*keyword* repair on the short name
(`MyPackage`→short `package`, a keyword) — left raw for the caller, pending a decision.

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
2. **Number verbalization** — `1 → One`, `12 → Twelve`, position-aware: **verbalize a leading numeral** (as the
   leading-digit repair, an alternative to the `X` prefix: `12 variable → TwelveVariable` rather than `X12Variable`)
   but **keep interior digits** (`variable 12 → Variable12`). Bounded and locale-aware; policy may instead **keep
   digits** even when leading for value-like targets (an HTTP status enum wants `Status200`, not `TwoHundred`).
   The decimal point is treated as a separator and **elided by default** (`index 0.1 → Index01`), *not* verbalized to
   `Dot`; full fraction verbalization (`OneTenth`) is opt-in via the `numbers` engine and reaches `GoMangler` only
   through the affix rule (`0.1 index → OneTenthIndex` when that mode is selected). This keeps the default separator
   predicate simple (it may use `unicode.IsPunct`, which elides `.` and `,`).
3. **Rune-name / phonetic fallback** — for tokens with no transliteration (emoji, exotic scripts), derive a word from
   the Unicode name (`golang.org/x/text/unicode/runenames`, e.g. 😀 → "grinning face" → `GrinningFace`). Opt-in, since
   it pulls a sizable generated table.
4. **Empty-result safeguard** — if policy would yield an empty identifier (input was a lone dropped marker), fall back
   up the layers (verbalize → phonetic) so we never emit `""`.

Verbalization policy is therefore a property of the **target**, not a global table: a `const`-name target verbalizes
aggressively, a field-name target drops leading markers, a doc-comment target may keep symbols literally. This is the
seam where v1's "everything goes through `ToGoName`" pressure is relieved.

#### 4.7.1 Asciification tiers (rune → ASCII/word) and the "never render" set

The rune-name fallback (§4.7 layer 3) is the general escape hatch, but most runes resolve more cheaply. Tiers, tried
in order:

1. **Latin diacritics** → the `asciiFold` map (fold to base, case-preserving; digraphs for `æ œ ß þ ð`).
2. **Decimal digits (`Nd`)** → ASCII value via a compact per-script *digit-zero offset* table
   (`'0' + (r - 0x0660)` for Arabic-Indic, etc.). Detect with `unicode.IsDigit` / `unicode.Is(unicode.Nd, …)`, which
   is `Nd`-**only** (verified): `Ⅶ` (Roman, `Nl`) and `½ ②` (`No`) return `IsDigit=false`, so they *automatically*
   fall through to tier 4. (Do **not** use `unicode.IsNumber`, which is `Nd+Nl+No` and would wrongly capture them.)
3. **Combining marks (`Mn`/`Mc`/`Me`)** → **stripped in place** (pure deletion; the base rune is untouched, nothing is
   renormalized). This pairs with tier 1 to cover *both* Unicode forms without `x/text/unicode/norm`: precomposed `é`
   → map; decomposed `e`+`◌́` → strip mark, base `e` survives. (An optional future NFD pass would only add coverage of
   exotic *precomposed* letters missing from the map, e.g. Vietnamese `ế`.)
4. **Everything else renderable** (non-Latin base letters, `Nl`/`No` numbers, unmapped symbols, single-codepoint
   emoji) → **rune-name fallback**, opt-in via `runenames` (`Ⅶ`→"…SEVEN", `Г`→"…GHE", `😀`→`GrinningFace`).
   Hard limit: **CJK unified ideographs have no phonetic name** (`中` = "CJK UNIFIED IDEOGRAPH-4E2D") → elide or
   placeholder. Hangul/Greek/Cyrillic/etc. have real names and work.

**The "never render" set** — elided even though `runenames` could name them:

| Category | What | Note |
|---|---|---|
| `Mn` `Mc` `Me` | combining marks | stripped (tier 3) — "COMBINING ACUTE ACCENT" is not a word |
| `Cc` `Cf` | control, format | ZWJ, ZWNJ, ZWSP, BOM, LRM/RLM, soft hyphen, tag chars |
| `Cs` `Co` `Cn` | surrogate, private-use, unassigned | no meaningful name |
| `Lm` `Sk` | modifier letters & symbols | standalone accents, backtick — already elided as separators |

Caveat: some of these (`ZWJ` `Cf`, variation selectors `Mn`, skin-tone modifiers `Sk`, tag chars `Cf`) are
*structural glue* inside emoji grapheme clusters — consumed by a future emoji decoder, not rendered standalone.

**Forthcoming enhancement (planned, not this iteration): grapheme clusters + extended Unicode properties.**
The value path will eventually segment by **grapheme cluster** so multi-codepoint units resolve as one name:
flag sequences (`🇮🇪` = regional-indicator pair `IE` → ISO-3166 → `Ireland`), ZWJ emoji (`👨‍👩‍👧` → family),
skin-tone/VS modifiers (`👍🏽`), tag flags. This needs an emoji/CLDR annotation table beyond per-codepoint
`runenames`; Fred has a generator (à la `mattn/go-runewidth`, which embeds Unicode attribute tables) to produce the
extended-property tables when we get there. Until then: **single-codepoint emoji via `runenames`; flags/sequences
resolve per-codepoint** (`🇮🇪` names its two indicators rather than "Ireland"). This matters mostly for `ValueMangler`
(enums, values), rarely for real Go idents (JSON-key-derived).

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

> Reconciled with the 2026-07-05 review (see §10). Key shape changes from the earlier sketch:
> no `.Go()` facade (concrete `GoMangler`); no `To(Target, …)` free entry — the composable core is
> `Mangler.Transform(TargetTransform, string)`; targets are **compiled immutable recipes**, presets are
> **functions**; the string `Transformer` tier is retired (all stages operate on `*Tokens`); `numbers`
> becomes its own subpackage; `Make`/`New` construction convention.

```go
package mangling // v2

// ─── Construction convention ───────────────────────────────────────────────
// MakeXxx returns a value; NewXxx returns a pointer. Every mangler is immutable
// after construction and safe for concurrent use (per-call scratch comes from a pool).

// ─── Tokenizer: segmentation only (opinionated, not user-pluggable yet) ─────
type Tokenizer struct { /* tokenOptions */ }
func MakeTokenizer(opts ...TokenOption) Tokenizer
func (t Tokenizer) Tokenize(s string) iter.Seq[string]   // convenience; materializes per-token strings
// internal: tokenize([]rune) iter.Seq[[]rune] — the zero-copy hot path used by everything else.
// Knobs: WithTokenSeparator(func(rune) bool); (maybe) WithLetterDigitBoundary(bool) for utf8/oauth2.
// No public SplitRule: the §4.2 boundary signals are opinionated per ruleset for this iteration.

// ─── Mangler: ruleset-neutral casing/inflection presets over TargetTransform ─
type Mangler struct { Tokenizer /* + options */ }
func MakeMangler(opts ...Option) Mangler
func NewMangler(opts ...Option) *Mangler

// The composable core — ONE mechanism, many outputs. A TargetTransform is a compiled, immutable
// recipe (casing × separator × affix × stages × repair). The mangler supplies the *data* the
// stages bind to at run time, so the same target degrades gracefully across manglers (§4.5).
func (m Mangler) Transform(t TargetTransform, s string) string

// Neutral preset methods — thin wrappers over Transform + a preset target:
func (m Mangler) Titleize(s string) string    // TargetTitle
func (m Mangler) Humanize(s string) string    // TargetSentence
func (m Mangler) Snakize(s string) string     // TargetSnake
func (m Mangler) Kebabize(s string) string    // TargetKebab
func (m Mangler) Camelize(s string) string    // TargetCamel
func (m Mangler) AllCaps(s string) string     // TargetAllCaps (was "Capitalize" → renamed)
func (m Mangler) Pluralize(s string) string   // shared inflection engine
func (m Mangler) Singularize(s string) string

// ─── TargetTransform: opaque, immutable recipe; presets are functions ───────
type TargetTransform struct { /* ALL fields unexported */ }
func MakeTargetTransform(opts ...TargetOption) TargetTransform

// Presets return a fresh immutable value (no exported mutable surface, impossible to corrupt).
// Named after the FORM they produce — no participles of neologized verbs (TargetCamel, not …Camelized).
func TargetTitle() TargetTransform
func TargetSentence() TargetTransform
func TargetSnake() TargetTransform
func TargetKebab() TargetTransform
func TargetCamel() TargetTransform
func TargetAllCaps() TargetTransform

// Target build options:
func WithCasing(c CasingByPosition) TargetOption  // word casing as a function of position
func WithSeparator(sep string) TargetOption        // "", "_", "-", " ", "."  (output separator)
func WithAffix(a AffixRule) TargetOption            // leading-non-letter repair: prefix|rune-name|strip|number-words
func WithRepair(r RepairRule) TargetOption          // reserved-word detection + repair token
func InjectStage(at Stage, st Transform) TargetOption // power hook: a raw closure over *Tokens

// ─── Stage model — the retired string tier lives on here as *Tokens closures ─
// A stage NEVER sees strings: it operates on the pooled zero-copy token model (position, kind,
// casing, split/merge). "Stateful vs stateless" is invisible — a stage is a pure function of its
// argument, capturing no mutable per-call state, so the same recipe runs concurrently.
type Transform func(*Tokens)  // was `Transformer func(string) string` — retired
type Stage int                // AfterSegment, AfterTransliterate, AfterInitialism, BeforeAssemble

// ─── GoMangler: Go ruleset — idents, packages, files, modules (enlarged scope) ─
type GoMangler struct { Mangler /* + NumberMangler; goOptions */ }
func MakeGoMangler(opts ...GoOption) GoMangler
func NewGoMangler(opts ...GoOption) *GoMangler

func (g GoMangler) IdentExported(s string) string    // was ToGoName
func (g GoMangler) IdentUnexported(s string) string  // was ToVarName; reserved-word repair here only (type→typeVar)
func (g GoMangler) Package(s string) (alias, pkg string)
func (g GoMangler) Module(s string) string
func (g GoMangler) File(s string) string             // neutralizes _test, _linux, _amd64, … suffixes
func (g GoMangler) DirName(s string) string
func (g GoMangler) JSONName(s string) string
func (g GoMangler) HumanName(s string, title bool) string

// Value → identifier convenience. ConstName ONLY — type-name prefixing for enum members is the
// codegen template's job (typeName + ConstName(value)), not the mangler's. (EnumName dropped.)
func (g GoMangler) ConstName(value string, opts ...ValueOption) string

// ─── Value → identifier: knob-driven, best-effort ───────────────────────────
// NOTE (2026-07-06): the standalone `ValueMangler` type was dropped. Verbalization is reached only through
// `GoMangler.ConstName(value, ...ValueOption)` — there is no separate value entry point (§11). The value knobs
// below are the *planned* option surface for ConstName; only WithGoNumberOptions is wired today.
// func OnSymbol(p SymbolPolicy) ValueOption      // drop | verbalize | phonetic, per (symbol, position)  [planned]
// func OnNumber(p NumberPolicy) ValueOption       // to-words | keep-digits | bounded                     [planned]
// func OnUnknownRune(p RunePolicy) ValueOption    // drop | rune-name | prefix                            [planned]
// func WithValuePrefix(word string) ValueOption   // leading-digit / empty-result safeguard               [planned]

// ─── package numbers: separate subpackage (own scope & complexity) ──────────
type NumberMangler struct { /* numberOptions */ }
func (m NumberMangler) NumberWords(s string) string           // "123" → "one hundred and twenty three"
func (m NumberMangler) AppendWords(dst []byte, s string) []byte // string-free sibling; caller pools dst → 0 alloc
func (m NumberMangler) DigitWords(s string) string            // "123" → "one two three"
func NumberWords[T Numerical](n T) string
func NumberOrdinal[T Numerical](n T) string          // 31 → 31st
func NumberRoman[T Integer](n T) string              // 6 → vi (codegen: compact loop indices)
// Digit-group reconstruction (1 234 → 1234) lives HERE, not in the tokenizer (keeps the scanner
// single-pass, lookahead-free). Fraction inference (0.25 → one quarter) is opt-in; consumed by
// ValueMangler and, via the affix rule, optionally by GoMangler.

// ─── Options (root) ─────────────────────────────────────────────────────────
func WithInitialisms(words ...string) Option            // replace the default set
func WithAdditionalInitialisms(words ...string) Option   // augment the default set
func WithReservedWords(words ...string) Option            // detection set (map[string]struct{}, not a repair map)
func WithTransliterator(t Transliterator) Option
func WithInflection(rules InflectionRules) Option
func WithRepairToken(tok string) Option                  // ruleset field (Go default "Var"); not a hardcode
```

## 6. Capabilities

> _To expand._ Per capability: motivation, plug shape, default, interaction with the pipeline.

- **Verbalization stack** (§4.7) — the layered, position-aware replacement for v1's flat `replaceFunc`:
  - *symbol policy*: per-`(symbol, position, target)` drop / verbalize / phonetic (fixes the `@id` quirk).
  - *number-to-words*: bounded, locale-aware cardinal verbalizer (`12`→`Twelve`); doubles as leading-digit repair;
    per-target "keep digits" escape (`Status200`).
  - *rune-name fallback*: opt-in `golang.org/x/text/unicode/runenames` for emoji/exotic runes.
- **ASCII folding** — diacritic folding (`é`→`e`, `ż`→`z`) as a **ruleset policy toggle**, not a hard rule.
  - *Motivation*: linter-clean output (`gosmopolitan`, `asciicheck`). **Not** a language requirement — Go's spec
    admits any Unicode letter in identifiers (`type Żaba struct{}` compiles), so folding is *always* a preference,
    never mandatory for any target.
  - *Plug shape*: a fold **stage** (runs before assembly, `é`→`e` unconditionally — orthogonal to casing, so it
    also folds lowercase bodies), toggled by one option (`WithAsciiFolding(bool)`). The capability is **neutral**
    (lives on `Mangler`, uses the `Ascii()` / `ToAscii()` helpers); only the *default* is ruleset-driven.
  - *Defaults are per-target policy*, same family as initialisms / reserved words ("what does clean Go output look
    like for this project"):
    - Go **machine-name** targets (idents, package/file/dir names) default folding **ON** — because the
      go-openapi/go-swagger ecosystem ships `gosmopolitan`-clean code. Fully overridable: a caller who doesn't run
      that linter sets `WithAsciiFolding(false)` and legitimately gets `Żaba` as a valid identifier.
    - **Human-facing** casing (`Titleize`, `Humanize`, `GoMangler.HumanName`) defaults **OFF** — preserve `Éric` /
      `Żaba`. Folding accents out of human text is a bug, and dropping them changes letter identity in
      Polish/Czech/Hungarian/Turkish (`Ż`≠`Z`, `ł`≠`l`).
  - *Out of scope as a default*: the narrow "drop accent on capitals **only**" (French typography, i.e.
    `Ascii(unicode.ToUpper(r))` — fold the cased rune, keep the body). It is neither faithful Unicode nor full ASCII;
    it is culture-bound, so it stays an **injectable stage** for the rare caller, never a core default or named option.
- **Inflection** — singular/plural engine absorbed from `go-openapi/inflect` (irregulars, uncountables, suffix
  rules). Shares the dictionary with initialism plural detection. Primary new use case: doc-comment generation.
  Closing `go-openapi/inflect` is in scope.
- **Locale-aware casing** — Turkish `i`, etc. Likely opt-in; default uses `unicode` simple casing.
- **Stopwords / abbreviation maps** — optional transforms (deferred unless needed).

## 7. Performance plan

> _To expand._ Matching algorithm (trie / Aho-Corasick vs current state machine); pooling strategy for `Tokens`;
> the alloc budget vs v1; benchmark continuity (carry the `BenchmarkToXXXName` suite forward as a regression gate).

**Alloc budget (measured, 2026-07-07).** The ident hot path (`IdentExported`/`IdentUnexported`, `Camelize`) is
**1 alloc/op** — a single pooled `Tokens` (zero-copy rune views) plus one assembly buffer. The number verbalizer
streams into one byte sink (`buf`) instead of building `[]string`+`strings.Join`: cardinals fill a stack `[7]int64`
of digit groups and write words through a shared buffer; the scanner is byte-based (no `[]rune` copy) and slices
number runs directly out of the input. Net: `NumberWords` is **0 alloc** for non-numeric input (fast path), **1**
for numeric; `ConstName` fell 5 → **1 alloc/op** over two passes. `NumberMangler.AppendWords(dst, s)` is the
string-free sibling — a caller that pools `dst` verbalizes **allocation-free** (differential-fuzzed against
`NumberWords` for parity).

## 8. Migration / v1 mapping

> _To expand._ Table mapping each v1 method + option to its v2 expression; note where output intentionally changes
> (all-caps, initialism boundaries) so downstream golden files are updated deliberately.

## 9. Open questions

Still open:

- `override` field vs side arena for rewritten content (transliteration/inflection allocation strategy).
- Repair token: `Var` is the Go ident default — is it right for *all* colliding idents (a `range` **type** → `RangeVar`
  reads oddly), or should type-position idents prefer a different token? (Deferred; single token for now.)
- Module-name rule: how much of "valid module path" belongs here vs. the future environment helper.
- Number verbalization bounds & locale: cap (e.g. ≤ 9999?) then fall back to digits + prefix? English-only first?
- Leading-marker default: is *drop* always right for `@`/`#`, or per-symbol (drop `@`, but `#`→`Hash`)?
- `runenames` dependency: opt-in transform only, to keep the core table-free — confirm it never lands in the default Go ruleset.

Resolved in the 2026-07-05 review (→ §10):

- ~~Facade shape~~ — no `.Go()`; a concrete `GoMangler` with an enlarged scope (idents, packages, files, modules).
- ~~Reserved-word repair strategy~~ — rule-based (detection set = keywords ∪ builtins + repair token); Go default
  token `Var`; per-word repair map deferred. **Unexported only** (`type`→`typeVar`); exported left undistorted
  (`Type`) — minimum-distortion over symmetry (revised 2026-07 — see §4.5).
- ~~Letter↔digit boundary / digit-group rule~~ — tokenizer stays opinionated & lookahead-free; digit-group
  reconstruction moves to `numbers`.
- ~~`To(Target, …)` vs facade~~ — the composable core is `Mangler.Transform(TargetTransform, string)`; targets are
  compiled immutable recipes, presets are functions; the string `Transformer` tier is retired.

## 10. Decisions locked — 2026-07-05 review

1. **One composable mechanism.** `Mangler.Transform(TargetTransform, string)` is the core; every preset method and
   every `GoMangler` output routes through it. No duplicated casing logic (the v1 sin).
2. **`TargetTransform` = compiled, immutable recipe** (casing × separator × affix × stages × repair), opaque
   (all fields unexported), built via `MakeTargetTransform(opts…)`. **Presets are functions** returning fresh values
   (`TargetCamel()`, …), named after the form (no `-ized` participles). `TargetAllCaps` replaces `TargetCapitalized`.
3. **Recipe vs data split.** Targets carry the recipe only; dictionaries (initialisms, keywords, reserved suffixes)
   live on the mangler and bind to stages at run time, so a target degrades gracefully across manglers.
4. **Stages operate on `*Tokens`, never strings.** `type Transform func(*Tokens)`. Zero-copy pooled token model
   (position/kind/casing, split/merge). Pure functions of their argument (no captured mutable state) → concurrency-safe;
   per-call scratch from a `sync.Pool`. The public string `Transformer` type is retired.
5. **Tokenizer stays opinionated.** No public `SplitRule` this iteration; only `WithTokenSeparator` (+ maybe a
   letter↔digit bool). The digit-group / thousands rule leaves the tokenizer for `numbers`.
6. **`numbers` becomes its own subpackage.** Full numeral surface (cardinals, ordinals, Roman, fractions, digit-group
   reconstruction). Roman/ordinal/fraction have real codegen consumers (compact loop indices, `OneQuarter` consts);
   gold-plating is spec-only for now, present to stress-test extensibility.
7. **Value path.** `ValueMangler.Verbalize` (knob-driven, best-effort) + a thin `GoMangler.ConstName` convenience that
   chains verbalize→ident. `EnumName` dropped — type-name prefixing is the codegen template's job.
8. **Construction convention.** `MakeXxx` → value, `NewXxx` → pointer; immutable after construction.
9. **Reserved words as sets** (`map[string]struct{}`), not a per-word repair map (deferred).
10. **Decimal point elided by default** (`index 0.1 → Index01`); fraction verbalization opt-in via `numbers`.
11. **ASCII folding is a ruleset policy toggle** (§6), never a hard rule (Go admits Unicode idents). Capability is
    neutral (on `Mangler`, via `Ascii()`); default is per-target — ON for Go machine-name targets (`gosmopolitan`-clean),
    OFF for human-facing casing (`Éric`/`Żaba` preserved), overridable via `WithAsciiFolding`. The French
    "drop-accent-on-capitals-only" variant is an injectable stage, not a default.
12. **Path-returning targets preprocess; tokenizer stays path-agnostic** (§4.6.1). `Package`/`Module` split on the
    last `/`, keep the prefix verbatim, and mangle only the basename — two ways: full path element for the import path
    (`@alpha-beta`→`at-alpha-beta`), last `-`-segment identifier for the package name/alias (`alpha-beta`→`beta`).
    Skip a trailing `/vN` when picking the name-bearer.
13. **Asciification tiers** (§4.7.1): Latin-diacritic map → `Nd` digit-offset (`IsDigit`, not `IsNumber`) → strip
    combining marks in place (no renormalization; pairs with the map to cover NFC+NFD dep-free) → `runenames` opt-in
    for the rest (CJK ideographs excepted). A defined **never-render set** (`Mn/Mc/Me`, `Cc/Cf`, `Cs/Co/Cn`, `Lm/Sk`)
    is always elided. **Graphemes + extended-Unicode tables** (flags→ISO-3166, ZWJ emoji, generator à la
    `go-runewidth`) are a **forthcoming enhancement**; for now single-codepoint emoji only.

## 11. Implementation status — 2026-07-06

Snapshot of the branch against this design. Legend: ✅ done & tested · 🚧 stub / partial · 📋 planned · ❌ dropped.

### Built and tested ✅

| Area | What landed | Design ref |
|---|---|---|
| Segmentation | `Tokenizer` with the §4.2 boundary signals; case-alternance is a first-class boundary (all-caps no longer explodes) | §4.2 |
| Token model | zero-copy `Tokens` over a shared rune slice, pooled; `Text`/`Kind`/`Casing`/`All`/`SetKind`/`Rewrite` | §4.3 |
| Assembly | `assemble` = casing × separator; `writeCased` initialism/word casing by position | §4.5 |
| Composable core | `Mangler.Transform(TargetTransform, string)`; `MakeTargetTransform`; presets **as functions** | §4.5, §10.1–2 |
| Presets | `TargetTitle/Sentence/Snake/Kebab/Camel/Pascal/AllCaps` + methods `Titleize/Humanize/Snakize/Kebabize/Camelize/Pascalize/AllCaps` (**`Pascal` added beyond the sketch**) | §5 |
| Initialisms | `initialismTrie`: whole-token, sub-token, adjacent-window merge (`IPv4`/`UTF8`), pluralized keys | §4.4 |
| ASCII folding | `foldASCII` stage, `foldToASCII` (diacritic map, digraphs), combining-mark strip; `WithAsciiFolding` | §4.7.1 tiers 1&3, §6 |
| GoMangler | `IdentExported`, `IdentUnexported`, `Package`/`PackageWithParts`, `Module`, `File`, `ConstName` | §4.5, §4.6.1, §5 |
| Go repairs | reserved-word (unexported only → `typeVar`), file-suffix (`_test`/GOOS/GOARCH → `swagger`), package/module short-name (`main`→`mainpkg`, `/v2`→`version2`) | §4.5, §4.6 |
| Go options | `WithGoDefaults`, `WithGoInitialisms`/`UseGoInitialisms`, `WithGoInitialismPlurals`, `WithManglerOptions`, `WithGoNumberOptions` | §5, §4.8 |
| numbers pkg | own subpackage: `NumberWords` (cardinals, fractions incl. 1/6·1/7·1/9, digit-group reconstruction, special numbers, `StripOne`/`StripAnd`/precision), `NumberRoman`; **Unicode numeral runes** (`No`/`Nl`: `½`,`Ⅶ`,`②`) via `RuneNumber` + rune-aware scanner; wired into `ConstName` | §4.7.2, §5, §10.6 |
| Leading-digit repair | `verbalizeLeadingNumber` verbalizes a leading numeral, keeps interior digits (`12 men`→`Twelve…`, `var 12`→`Var12`) | §4.7.2 |

### Stub / partial 🚧 — the remaining work

| Item | Current state | Design ref |
|---|---|---|
| **Unicode rune-naming** | ✅ **Done.** `v2/runewords/` generator + table (below); `ToAscii`/`UnicodeName`/`Ascii` implemented and wired into the asciify tier + `ConstName` (`expandRuneNames`, §4.7.1 tier 4). | §4.7.1 tiers 2&4, §4.7 layer 3 |
| **Inflection engine** | `Pluralize`/`Singularize` `return ""`; `Conjugate` commented out. The `go-openapi/inflect` absorb has not started; initialism plurals currently carry their own precompute rather than sharing an engine. | §6, §4.4 |
| **Value-policy knobs** | `ConstName` accepts `...ValueOption` but ignores them (`_ = opts`). `OnSymbol`/`OnNumber`/`OnUnknownRune`/`WithValuePrefix` unbuilt; symbol policy (`@id`→drop) not implemented. | §4.7 layer 1, §5 |
| `Tokens.Split`/`Merge` | present but `TODO(#3)` — not needed by any built stage yet. | §4.3 |
| Target build options | only `WithSeparator` exists; `WithCasing`/`WithAffix`/`WithRepair`/`InjectStage` from the §5 sketch are not exposed (presets cover current needs). | §5 |

### Rune-naming prototype — `v2/runewords/` (2026-07-06)

Own subpackage (like `numbers`) for dependency isolation — the table **always links** (asciification is a core
feature; decided 2026-07-07, §12), it is not opt-in. `gen.go` (`//go:build ignore`) builds a compact `rune → word`
table from `ucd/DerivedName.txt`; `lookup.go` exposes `Word(rune) (string, bool)`.
Pipeline, in the sequence agreed with Fred:

1. **Exclude what other layers already handle or elide** — ASCII, Latin+diacritics (fold map), digits (`Nd`),
   combining marks, controls/format, separators/spacing-modifiers.
2. **Exclude drop-during-asciify classes** — CJK Han, Hangul syllables (algorithmic romanizations; Hangul alone is
   11,737 lines), and a curated list of **decorative/technical symbol blocks** (box drawing, block elements,
   geometric shapes, braille, control pictures, Misc Technical/Symbols, Dingbats, Yijing, musical, mahjong/domino,
   legacy-computing) — **gated by `Extended_Pictographic`** (from `ucd/emoji-data.txt`) so real emoji inside mixed
   blocks survive (`❤`→Heart, `✈`→Airplane, `⌚`→Watch, `☯`→YinYang) while non-emoji decoration (`✓ ⌂ ─`) is elided.
3. **Extract the distinctive remainder** — strip taxonomy (letter/syllable/character/sign/number templates + a
   leading **script-token** strip from `unicode.Scripts`), then apply **one collapse rule** embodying "one readable
   word is enough": ≤2-word remainders are kept whole (`GrinningFace`, `ThumbsUp`, `KoKai`), 3+ word phrases reduce
   to their most-distinctive token — the longest word that is neither glue nor a qualifier (color/weight/orientation
   stoplist), so `heavy black heart`→`heart`, `place of sajdah`→`sajdah`, `fehu feoh fe f`→`fehu`.
4. **Compact index** — interned word blob (`directData` trick on *every* entry: short words dedupe hard, 994×
   `"mathematical"`, 109× `"a"`), plus the compacted lookup structure (**§12**): keys are **interval-encoded** as 740
   maximal runs of consecutive codepoints (`runStart`/`runFirstIndex`) — a single binary search then arithmetic, no
   scan; offsets are **18-bit** (`wordOffLo` uint16 + `wordOffHi` 2-bit sidecar); `nameWordID` maps rune position → word
   id. The flat `nameRunes []rune` and `wordOffsets []uint32` are gone.

Real numbers (Unicode 15.0, 44,115 named codepoints): **23,191 kept (53%)**; the table is **~173 KiB**
(286 KiB → 178 via the §12 compaction → 173 after pruning the 1,151 `No`/`Nl` numerals, now routed to the `numbers`
engine), still **~7× smaller** than x/text/runenames' 1.3 MB for full names (blob 101 KiB, vocabulary ~11,000 words).
The aggressive collapse eliminated the verbose tail entirely (every kept remainder is now ≤2 words). Sample idents:
`α`→`Alpha`, `ж`→`Zhe`, `😀`→`GrinningFace`, `👍`→`ThumbsUp`, `❤`→`Heart`, `۩`→`Sajdah`, `€`→`Euro`;
box-drawing/braille/geometric/non-emoji decoration elided; `½`/`Ⅶ`/`②` routed to `numbers` (§4.7.2).

**Wired into the mangler (2026-07-06):** rune-naming is a **neutral `Mangler` capability**, not Go-specific. The
string-level `expandRuneNames` pass runs via `Mangler.asciifyInput` *before* segmentation — shared by
`Mangler.Transform` (so every preset — `Camelize`, `Snakize`, … — names runes when folding is on) and by
`GoMangler.identifier`. Running before segmentation lets a multi-word name re-segment and re-case per word
(`😀`→`GrinningFace`, not "Grinning face"); diacritics stay in the zero-alloc token-fold stage; CJK/decorative runes
drop to a separator. Gated by the same `asciify` option as diacritic folding. `ConstName` inherits it via
`IdentExported`. Verified across both manglers: `σ value`→`SigmaValue`/`sigma_value`, `π`→`Pi`,
`δ plus ε`→`DeltaPlusEpsilon`, `日本 value`→`Value`. (`TestGoManglerRuneNames`, `TestAsciiUtilities`, harness CJK case.)

**Remaining tuning (not blockers):** the qualifier stoplist size. Compaction is **done** (§12: 286 → 178 KiB) and the
table is **always linked** (not opt-in). Number-class routing is **done** (No/Nl → `numbers`, §4.7.2); grapheme
sequences (flags, ZWJ emoji) remain tracked in the **§12 backlog**.

### Dropped / superseded ❌

- **`ValueMangler` type** — folded entirely into `GoMangler.ConstName`; no separate value entry point (§5 note).
- **`EnumName`** — type-name prefixing is the codegen template's job (already recorded §10.7).
- **`split_rule.go` helpers** (`splitRuleCaseAlternance`/`Separators`/`DigitGroups`) — dead `TODO` stubs; the tokenizer segments internally.

### Not built — scope undecided 📋

`DirName`, `JSONName`, `HumanName`, `NumberOrdinal`, a public `DigitWords` method — all sketched in §5 but not implemented and not yet confirmed in scope. `NumberOrdinal` in particular is parked (likely ditch). Decide per need before building.

## 12. Backlog & review — 2026-07-07

Reassessment pause after the alloc-reduction and rune-naming work. Groups the remaining path to graduation.

### Decisions locked this round

1. **`numbers` stays a subpackage — won't lift into core.** Its number-aware scanner (it must see decimal points and
   digit-group separators the general tokenizer elides) has proven an effective, self-contained interface. No merge.
2. **`buf` stays concrete — won't genericize over a `sink` type-param to avoid the one `unsafe.String`.** The single
   `unsafeStr` (fresh buffer, never mutated after — same invariant as `strings.Builder.String()`) keeps `NumberWords`
   at 1 alloc and `AppendWords` at 0 when pooled. When the linter (`gosec`, `default: all`) flags it, add a scoped
   `//nolint` with the invariant as its comment — cheaper and clearer than a generic indirection.

### Remaining work items (path to graduation)

| Item | Kind | Notes |
|---|---|---|
| **Inflection engine** | feature (orthogonal) | `Pluralize`/`Singularize`/`Conjugate`; absorb `go-openapi/inflect`; share with initialism plurals (§6, §4.4). |
| ~~asciify: route number-class runes through `numbers`~~ | ✅ **done (2026-07-07)** | `No`/`Nl` (`½`, `Ⅶ`, `②`) now route to `numbers`. Table `numbers/numerals.go` (`map[rune]float64`, 1,151 runes from `DerivedNumericValues.txt`, `Nd`/`Lo` excluded) via `numbers.RuneNumber`. **Three treatments:** the `numbers` engine spells the value (`ConstName("½")`→`OneHalf`), the asciify tier renders a plain number (`ToAscii("½")`→`"0.5"`, 3-decimal cap), `UnicodeName` elides. `No`/`Nl` pruned from the `runewords` Word table (178→173 KiB). `fractionBases` gained 1/6, 1/7, 1/9 so every standard vulgar fraction spells cleanly. |
| **asciify: grapheme support** | enhancement | Flags (→ISO-3166), ZWJ emoji sequences — the parked §4.7.1 work; currently single-codepoint only. Needs additional UCD data (emoji-sequences / region-indicator). |
| **Fuzz tests** | test | Add targets; name the invariants explicitly: differential parity (have it for `AppendWords`≡`NumberWords`) **plus idempotency** `f(f(x)) == f(x)` for every identifier producer (`ToGoName`/`ConstName`/`VarName`/`FileName`). |
| **Coverage → 85%+** | test | A few uncovered paths remain; close them. |
| **Concurrency test** | test | Explicit `-race` test hammering `Transform`/`ConstName` from N goroutines — closes the §10.4 "concurrency-safe" claim. |
| **`GoIdent*` empty-result edge case** | correctness | Input that fully reduces to separators/elided runes yields an incorrect empty string. **Decide the contract once and apply it uniformly** across `ToGoName`/`ConstName`/`VarName`/`FileName` (empty vs. `_` sentinel vs. error) — not a per-function patch. |
| **README + docstrings** | documentation | Beef up README (started); **explicit v1 differences** (case-alternance boundary — [go-openapi/swag#123](https://github.com/go-openapi/swag/issues/123)); comprehensive docstrings; better-documented options. |
| **Productize the UCD codegen** | documentation / tooling | See "provenance" below. |
| **asciify toggle granularity** | open design decision | Single `asciify` flag vs. separate fold-diacritics / name-runes toggles. Shapes public API — resolve **before** graduation. (Was §9 open. Purely an API-shape call now: the runewords table always links regardless — decided 2026-07-07.) |
| **v1→v2 comparative benchmark** | perf / doc | Not just standalone benches — a v1-vs-v2 table feeds the "explicit v1 differences" doc and the migration story. |

+ add unicode v17 files and verify the generator for those (prepare for go1.27 support next month).

### UCD codegen provenance (part of "productize")

`gen.go` records **no Unicode version** and no source checksums. For a table meant to be a stable failsafe, emit into
the `tables.go` header: the UCD version, a `//go:generate` line, and source-file checksums — so regeneration is
reproducible and data drift is detectable. This is the load-bearing part of productizing the codegen.

### Rune-name table: compaction — LOCKED plan (2026-07-07 spike)

Every candidate was measured on the real data (24,235 kept runes, Unicode 15.0) via throwaway in-package harnesses
before committing. Two independent encodings win; both keep the table pure static `.rodata` (no heap, no init-alloc,
no `unsafe`, no `deflate`).

**Starting footprint:**

| Segment | Bytes | Role in lookup |
|---|---:|---|
| `nameRunes` (`[]rune`) | 96,940 | ① keys — sorted rune set + rank |
| `wordBlob` (`string`) | 102,901 | ⑤ interned distinct words (11,098) |
| `nameWordID` (`[]uint16`) | 48,470 | ③ rune position → word id |
| `wordOffsets` (`[]uint32`) | 44,396 | ④ word id → blob slice |
| **total** | **292,707 (285.8 KiB)** | |

**Decision 1 — keys: interval (range) encoding, replaces `nameRunes`.** The kept codepoints are near-contiguous
(measured: 99.7% of consecutive deltas are 1), so the sorted set collapses to **740 maximal runs** of consecutive
runes. Store `runStart []uint32` (740) + `runFirstIndex []uint32` (741, +sentinel). Lookup is a single binary search
over `runStart` (~10 steps) then pure arithmetic `pos = runFirstIndex[i] + (r − runStart[i])`, bounds-checked against
`runFirstIndex[i+1]` (catches runes in a gap). **No linear scan, no popcount** — this beat blocked-varint (26 KiB,
≤63 scan), Elias–Fano (13 KiB, select/rank machinery), high/low prefix split (25 KiB), and per-page bitmap-hybrid
(5.4 KiB but needs popcount). **95 KiB → 5.9 KiB.** (Measured alternatives kept in the design history; interval
encoding won on size × simplicity × lookup cost jointly.)

**Decision 2 — offsets: 18-bit via `uint16` base + 2-bit sidecar, replaces `wordOffsets`.** Offsets need only 17 bits
(blob 103 KiB); 18 bits (256 KiB ceiling) gives Unicode-17/Go-1.27 headroom **and removes any need for banking** (the
banking-to-`uint16` idea was superseded — 18 bits addresses the whole blob, so a single global blob keeps full dedup,
zero cross-bank duplication). Implemented as `wordOffLo []uint16` (low 16 bits) + `wordOffHi []byte` (high 2 bits,
packed 4/byte, ~2,775 B) rather than a fully-packed 18-bit stream — same byte count, but byte-aligned reads (no
4-byte-spanning mask, no trailing pad). Reader: `off = uint32(hi)<<16 | uint32(lo)`. **44.4 KiB → 24.4 KiB.**

**Result:**

| Segment | Before | After |
|---|---:|---:|
| keys (`nameRunes` → `runStart`+`runFirstIndex`) | 96,940 | 5,924 |
| offsets (`wordOffsets` → `wordOffLo`+`wordOffHi`) | 44,396 | ~24,400 |
| `nameWordID` (unchanged) | 48,470 | 48,470 |
| `wordBlob` (unchanged) | 102,901 | 102,901 |
| **total** | **285.8 KiB** | **~177 KiB (−38%)** |

Both changes are pure static tables (no `unsafe`), lookup stays the 5-step chain (① binary-search range → ② arithmetic
→ ③ `nameWordID` → ④ 18-bit offset → ⑤ blob slice). Generator emits the runs + split offsets; a round-trip test
(pack→read == original) plus the existing rune-naming parity tests guard correctness.

**Not pursued** (recorded, diminishing returns past ~177 KiB):
- `nameWordID` (48 KiB) bit-pack to 14-bit → ~−6 KiB; consecutive runes have unrelated word-ids so no run structure.
- `wordBlob` (103 KiB) suffix-merge → ~−10–20 KiB at real generator complexity.
- "id-becomes-offset" (drop `wordOffsets`, per-rune offset) → ~−6 KiB more than Decision 2 but needs per-word length
  bytes (~+12 KiB) and a length-scan — net worse trade than the 18-bit sidecar.
- **`deflate` + inflate-at-init** — rejected: destroys the static-`.rodata`, zero-alloc property.

**Decision (2026-07-07): the table always links — not opt-in.** Asciification has become a *major* feature of the
package, so the Unicode data loading unconditionally is accepted as a core cost, not something to gate behind a build
tag or a separate import. At ~178 KiB the whole failsafe table is already **~7× smaller than golang.org/x/text/unicode/
runenames' 1.3 MB** (and far more useful here — collapsed identifier words, not full formal names). No opt-in
machinery; simplicity wins. (This retires the earlier "make it opt-in" lever and removes the linking-cost argument from
the asciify-toggle-granularity item — that item now stands or falls on API shape alone.)

---

## Appendix: design intent (raw notes)

> Slot reserved for Fred's intent write-up (the lost text). Paste here; I'll reconcile it into the sections above.
