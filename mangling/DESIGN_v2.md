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

**Exported and unexported identifiers are the *same* target**, differing only by the initial-rune case rule. Every
other decision — segmentation, transforms, and especially the reserved-word repair (§4.6) — is shared, so the two
outputs stay pure case-variants of each other. Because all Go keywords and predeclared identifiers are lowercase, the
exported form never collides on its own; deciding repair per-target independently would desync them (`Unexported("type")`
→ `typeVar` while `Exported("type")` → `Type`). We therefore decide repair on the shared, case-insensitive basis and apply
it to both: `typeVar` / `TypeVar`. Symmetric repair is the **default**; an option may decouple it for callers who prefer the
exported form left un-repaired.

The repair **token** is a ruleset field (§4.6). The Go ruleset defaults to the word suffix `Var` (matching
go-swagger's current behavior, easing migration); the token is a plain word rather than a trailing `_`, but the
symmetry argument is unchanged — `typeVar` and `TypeVar` remain pure case-variants regardless of the token.

### 4.6 Validate / repair (the consolidation seam)

After assembly, a target may run a **validity check with a repair strategy** — this is where go-swagger's bolted-on
rules move in. Each is *detect collision → mutate*:

| Target | Check | Default repair |
|---|---|---|
| identifier | not a Go keyword/predeclared (also builtins for unexported); valid identifier | word suffix (`type`→`TypeVar`) |
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
3. **`/vN` major-version elements**: a trailing `/v2` is a version, not the name-bearer — skip it when choosing the
   basename to name from (`github.com/user/repo/v2` → name from `repo`).
4. **Bare input** (no `/`): basename = whole string, empty prefix; falls out naturally.

So `Package`/`Module` are thin path-aware compositions over the primitives (split → mangle basename two ways → rejoin
with `/`), and the tokenizer stays single-purpose.

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
func (g GoMangler) IdentUnexported(s string) string  // was ToVarName; case-variant, shared repair (TypeVar/typeVar)
func (g GoMangler) Package(s string) (alias, pkg string)
func (g GoMangler) Module(s string) string
func (g GoMangler) File(s string) string             // neutralizes _test, _linux, _amd64, … suffixes
func (g GoMangler) DirName(s string) string
func (g GoMangler) JSONName(s string) string
func (g GoMangler) HumanName(s string, title bool) string

// Value → identifier convenience. ConstName ONLY — type-name prefixing for enum members is the
// codegen template's job (typeName + ConstName(value)), not the mangler's. (EnumName dropped.)
func (g GoMangler) ConstName(value string, opts ...ValueOption) string

// ─── ValueMangler: arbitrary literals → words (best-effort, knob-driven) ────
type ValueMangler struct { Tokenizer /* + valueOptions */ }
func MakeValueMangler(opts ...ValueOption) ValueMangler
func (v ValueMangler) Verbalize(s string) string
// Value knobs — the caller owns policy for gibberish input:
func OnSymbol(p SymbolPolicy) ValueOption      // drop | verbalize | phonetic, per (symbol, position)
func OnNumber(p NumberPolicy) ValueOption       // to-words | keep-digits | bounded
func OnUnknownRune(p RunePolicy) ValueOption    // drop | rune-name | prefix
func WithValuePrefix(word string) ValueOption   // leading-digit / empty-result safeguard

// ─── package numbers: separate subpackage (own scope & complexity) ──────────
type NumberMangler struct { /* numberOptions */ }
func (m NumberMangler) NumberWords(s string) string  // "123" → "one hundred and twenty three"
func (m NumberMangler) DigitWords(s string) string   // "123" → "one two three"
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
- ~~Reserved-word repair strategy~~ — rule-based (detection set + repair token); Go default token `Var`; per-word
  repair map deferred; symmetry preserved (`typeVar`/`TypeVar`).
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

---

## Appendix: design intent (raw notes)

> Slot reserved for Fred's intent write-up (the lost text). Paste here; I'll reconcile it into the sections above.
