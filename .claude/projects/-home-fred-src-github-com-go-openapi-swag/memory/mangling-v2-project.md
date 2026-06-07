---
name: mangling-v2-project
description: Goals and constraints for the mangling v2 redesign (started 2026-06-05)
metadata:
  type: project
---

Designing a v2 of the `mangling` module (name mangling for codegen: identifiers, file names, JSON keys).
Work started 2026-06-05 on branch `exp/mangling-v2`.

Key framing decided with the user:
- This is **temporary exploratory work**. v2 will most likely **move to its own repo** and be elevated to a
  **core reusable package** shared across all go-openapi/go-swagger codegen modules — no longer "a submodule of swag".
- v2 overall will break up swag, keep/improve what's useful, add missing parts. Mangling is the first piece.
- **No backward-compat constraints** on the v2 API. v1 mangling stays maintained for a long time (separate). If released
  from here it would just be a `v2.x.y` tag.
- Scope drivers (all four in): correctness fixes, API composability, new capabilities, performance/internals.
- Target breadth: **Go-centric but pluggable** — Go naming (revive initialisms) is first-class, design the core so
  other-language rulesets can plug in later. Not language-agnostic from day one.
- Working mode: **design doc first**, iterate before code.

## Consolidation mandate (new-capabilities driver)
v2 absorbs Go naming know-how currently dispersed in **go-swagger** `generator/internal/language/golang.go`
(`GolangOpts`/`Options` wrapping the mangler):
- Reserved words = the 25 Go keywords (identifier collision avoidance).
- Reserved file-name suffixes (`goOtherReservedSuffixes`): all GOOS + all GOARCH + `test`; `defaultGoFilenameFunc`
  appends `swagger` when the last `_`-segment collides (avoids accidental build-constrained files).
- Reserved dir names: `vendor`/`internal` → suffixed `_swagger`.
- Also wanted: package-name rule and module-name rule.
- Scope line: NAMING rules (valid package/module/file names) move into v2; environment/filesystem resolution
  (tryResolveModule, GOPATH exploration, base-import computation) stays in go-swagger, out of the mangler.

Also fuse **inflection**: `go-openapi/inflect` (Ruleset: Pluralize/Singularize/irregulars/uncountables/acronyms/
Humanize/Titleize/Underscore/Dasherize/Asciify) is largely redundant with the mangler — its "acronyms" ≈ our
initialisms, its casing fns ≈ our assembly presets. v2 absorbs inflection (use case: go doc generation), then
go-openapi/inflect is **archived/closed**. One plural engine shared with initialism plural detection (IDs→ID).

## Names vs values (key reframe from Fred's intent notes)
Two distinct input classes: word-like **names** (segment→case→initialism) vs arbitrary **values** (numbers, symbols,
emoji → const names) which need **verbalization**. v1's quirks come from forcing everything through ToGoName with one
flat position-blind rune→word table (every `@`→`At`, so JSON-LD `@type`→`AtType`). v2 verbalization = position-aware
layered transform over segmented tokens: (1) symbol policy `(symbol,position,target)`→drop/verbalize/phonetic (leading
`@`→drop→`Type`; interior→verbalize), (2) number-to-words `12`→`Twelve` (doubles as leading-digit repair vs `X` prefix;
per-target "keep digits" escape e.g. HTTP `Status200`), (3) opt-in rune-name fallback via `x/text/unicode/runenames`
(emoji→`GrinningFace`), (4) empty-result safeguard. Policy is a property of the target, not global.

## Design doc location
Canonical/polished: `~/.claude/plans/mangling-v2-design.md` (has §8 migration table, perf bar, reconciled appendix).
Working copy on branch: `mangling/DESIGN_v2.md` (slightly behind). Prefer the plans copy if they drift.

## Decisions locked so far
- Segmentation disjoint from initialism recognition (inversion of v1 trap). Boundary signals: separators, case
  alternance (lower→Upper, Upper-run→lower w/ 1-rune lookback), letter↔digit, script/category change.
- Token = zero-copy view into one shared []rune; pooled mutable Tokens slice; transforms are closures mutating in place;
  strings materialized only at assembly. Pipeline: segment→[transliterate·fold·initialism·inflect·verbalize]→assemble
  (casing×separator×affix)→validate/repair. Transform order opinionated per ruleset; callers inject at named stages.
- Ruleset = data (initialisms, reservedWords, reservedSuffixes, reservedDirs, inflection, repairToken) + named targets.
- Exported & Unexported = ONE target differing only by initial-rune case; reserved-word repair decided on shared
  case-insensitive basis → stay pure case-variants (`type_`/`Type_`). Default repair = trailing underscore. Symmetric
  by default, decoupling optional. Facade naming: `m.Go().Exported()/.Unexported()` (GoNameMangler), not ToGoName/ToVarName.
- Mangler immutable after construction (no AddInitialisms mutation).
- **Ownership asymmetry**: names are self-driven (system owns policy, strong defaults, bare `(string)→string` methods);
  values are knob-driven (caller owns policy, best-effort on gibberish). API reflects it: value methods `ConstName`/
  `EnumName` take `...ValueOption` (OnSymbol/OnNumber/OnUnknownRune/WithValuePrefix); name methods take just a string.

v1 documented limitations to fix: all-caps explodes ("THIS_IS_ALL_CAPS" → "t_h_i_s..."), fragile initialism-boundary
heuristics (IDS/IDx/IDs), English-only hardcoded pluralization, no Unicode→ASCII transliteration, bespoke ToXXX methods
(no composable casing×separator×word-transform), duplicated casing logic, value-type-with-pointer-index ownership.
v1 perf bar to not regress: ~3 allocs/op for ToGoName (after PR #106).
