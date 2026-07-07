# Go identifiers

The `GoMangler` turns any string into names that a Go code generator can safely emit:
identifiers, constant names, file names, and package or module path elements.

Its central guarantee:

> **Any input produces a valid, non-empty Go identifier.** This holds for every string —
> keywords, digits, symbols, emoji, non-Latin scripts, or the empty string. It is enforced
> by construction and proven by fuzzing (see [Tests](#always-valid-by-construction)).

```go
g := mangling.MakeGoMangler()

g.IdentExported("sample text")   // "SampleText"
g.IdentUnexported("sample text") // "sampleText"
```

## The rules being honored

A Go identifier must:

- start with a letter (a Unicode letter or `_`);
- continue with letters or Unicode decimal digits;
- not collide with a reserved word of the language.

If the first letter is upper-case the identifier is exported ("public").

On top of the compiler rules, the `GoMangler` also follows the conventions that Go linters
enforce, so its output reads as idiomatic, hand-written Go:

- camelCase / PascalCase over `snake_case` (no underscores);
- well-known acronyms stay upper-cased — *initialisms* (`revive`);
- ASCII-only output, even from non-Latin input (`gosmopolitan`, `asciicheck`) — see
  [asciification.md](asciification.md);
- names do not shadow a Go builtin.

## `IdentExported`, `IdentUnexported`

These work like [`Camelize`](../README.md) / Pascalize with Go-specific repairs layered on.

| Input | `IdentExported` | `IdentUnexported` | Why |
|---|---|---|---|
| `sample text` | `SampleText` | `sampleText` | words joined, cased for visibility |
| `user_id` | `UserID` | `userID` | `id` is an initialism |
| `IPv4 config` | `IPv4Config` | `ipv4Config` | mixed-case initialism preserved |
| `type` | `Type` | `typeVar` | keyword collision repaired (only the unexported form collides) |
| `@type` | `AtType` | `atType` | leading symbol verbalized |
| `123 lives` | `OneHundredAndTwentyThreeLives` | `oneHundredAndTwentyThreeLives` | a leading digit is illegal, so the number is spelled out |
| `中文` | `Empty` | `empty` | CJK elided → the empty-input fallback keeps the result valid |

The repairs, in order:

- **Keyword / builtin collisions** get a suffix (default `Var`: `type` → `typeVar`). The suffix
  is configurable with `WithGoReservedSuffix`. Note that only the *unexported* form can collide —
  every Go keyword and builtin is lower-case, so the exported `Type` is already safe.
- **A non-letter start** is verbalized so the identifier begins with a letter: digits get their
  number wording, symbols get a short word.
- **An identifier that reduces to nothing** (empty input, or input made entirely of separators /
  elided runes) falls back to a word — `Empty` / `empty` by default, configurable with
  `WithGoIdentFallback`.

Initialisms are configurable: `DefaultInitialisms()` is the built-in set; `WithGoInitialisms`
adds to it, and `UseGoInitialisms` replaces it wholesale.

## `ConstName`

`ConstName` produces an exported identifier, but with **every number verbalized** — the natural
form for enum values and named constants.

```go
g.ConstName("read only")   // "ReadOnly"
g.ConstName("status 200")  // "StatusTwoHundred"
g.ConstName("0.25")        // "OneQuarter"
g.ConstName("50%")         // "FiftyPercent"
```

Where `IdentExported` only spells out a number when it is *forced* to (a leading digit),
`ConstName` spells out all of them, everywhere in the string. Number rendering (special names,
precision, ...) is configurable through `WithGoNumberOptions` — see [numbers.md](numbers.md).

## `File`

`File` produces a `snake_case` file name and repairs stems that Go would treat specially.

```go
g.File("MyModel")       // "my_model"
g.File("test.go")       // "test_swagger.go"     — "test" stem would be a test file
g.File("config_linux")  // "config_linux_swagger" — "_linux" is a GOOS build constraint
g.File("café résumé")   // "cafe_resume"
```

A stem ending in a `_test` marker or a `GOOS`/`GOARCH` suffix would silently change how the Go
toolchain treats the file, so it gets a repair suffix (default `swagger`, configurable with
`WithGoFileRepairSuffix`). Directory prefixes and the file extension are carried through verbatim.

## `Package`, `Module`

`Package` maps a path to a short package name and a full import path; `PackageWithParts` also
returns the name's segments. `Module` neuterizes the elements of a module path that would not be
`go get`-able.

```go
short, pkg := g.Package("github.com/go-redis/redis") // "redis", "github.com/go-redis/redis"
short, pkg = g.Package("main")                       // "mainpkg", "mainpkg" — "main" is reserved
short, pkg = g.Package("xxxx/v2")                    // "version2", "xxxx/version2" — bare version element

g.Module("github.com/user/repo/v2")  // "github.com/user/repo/version2"
g.Module("example.com/internal")     // "example.com/internalpkg" — reserved dir name
```

Reserved package names (`main`, `internal`, `vendor`, `testdata`), Windows device names, and
bare major-version elements (`v2`) are all repaired so the result names a package that can
actually be imported.

## Always valid, by construction

The valid-identifier guarantee is not a best effort — it is the invariant the package is fuzzed
against. A fuzz target feeds arbitrary bytes through the identifier producers and asserts, with
`go/token.IsIdentifier`, that the output is always a legal Go identifier (and, in folding mode,
that its export visibility matches). v1 could emit invalid identifiers for some inputs; v2 cannot,
for any input.
