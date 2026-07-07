# Differences with v1

This package supersedes `github.com/go-openapi/swag/mangling` (v0.x). v1 remains maintained and
frozen; v2 is a clean reimplementation with the same job and a stricter, faster core.

There are no backward-compatibility constraints between the two — the API is new.

## Why a v2

v1 works well and is fast, but every documented v1 bug traces back to one root cause: **case
alternance was never modeled as a word boundary.** Lacking it, the initialism matcher became the
de-facto segmenter, fusing segmentation and initialism recognition — so neither could be fixed
without breaking the other (the "initialism trap").

v2 separates the concerns: **segment → transform → assemble → validate**, with case transitions as
first-class boundaries, and a distinct path for *names* (segmented) versus *values* (verbalized).

## Behavioral differences

| Case | v1 | v2 |
|---|---|---|
| All-caps runs | `ToFileName("THIS_IS_ALL_CAPS")` shatters into `t_h_i_s_...` | `Snakize` → `this_is_all_caps` (case *transition* is the boundary) |
| Non-ASCII input | leaks Unicode into identifiers (trips `asciicheck` / `gosmopolitan`) | folded to ASCII: `café` → `Cafe`, non-Latin romanized — see [asciification.md](asciification.md) |
| Numbers in values | a bare number produced an unusable identifier | verbalized: `ConstName("42")` → `FortyTwo`, leading digits spelled in `Ident*` |
| Emoji / symbols | dropped or mangled | named: `😀` → `GrinningFace` |
| Concurrency | `AddInitialisms` mutated shared state — not concurrency-safe | immutable value manglers, configured at construction; safe for concurrent use |

## API mapping

The manglers are now immutable values built with functional options (`MakeXxx` returns a value,
`NewXxx` a pointer), instead of a mutable `NameMangler`.

| v1 | v2 | Notes |
|---|---|---|
| `ToGoName` | `GoMangler.IdentExported` | exported identifier |
| `ToVarName` | `GoMangler.IdentUnexported` | unexported identifier |
| `ToFileName` | `GoMangler.File` | now also repairs build-constrained stems |
| `ToCommandName` | `Mangler.Kebabize` | kebab-case |
| `ToHumanNameLower` | `Mangler.Humanize` | sentence case |
| `ToHumanNameTitle` | `Mangler.Titleize` | title case |
| `Camelize` | `Mangler.Camelize` | unchanged in spirit |
| `AddInitialisms` (mutation) | `WithGoInitialisms` (option) | construction-time, concurrency-safe |
| `WithInitialisms` | `UseGoInitialisms` | replace the initialism set |
| `WithAdditionalInitialisms` | `WithGoInitialisms` | add to the set |
| `WithGoNamePrefixFunc` | *(automatic)* + `WithGoIdentFallback` | leading non-letters are verbalized without a custom hook |

New in v2, with no v1 equivalent:

- `GoMangler.ConstName` — exported names with every number verbalized (enum values);
- `GoMangler.Package` / `PackageWithParts` / `Module` — package and module path naming;
- `ToASCII` / `RuneToASCII` / `RuneShortName` — folding as plain functions;
- the [`numbers`](numbers.md) engine — cardinals, fractions, digit-group reconstruction.

## Migration notes

- Build one mangler and reuse it. A `GoMangler` is an immutable value; sharing it across goroutines
  is safe, and there is no per-call mutation.
- ASCII folding is **on by default** in the `GoMangler` (it was absent in v1). If you relied on
  Unicode passing through, opt out with `WithASCIIFolding(false)`.
- Struct-tag-based JSON field naming (v1's `ToJSONName` territory) lives in the separate `jsonname`
  package; the mangler's job is name casing, not tag inference.
