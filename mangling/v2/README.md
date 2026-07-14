# mangling

Turn arbitrary strings into valid Go identifiers, and apply common recasing operations
(camelCase, kebab-case, snake_case, ...).

It supersedes `github.com/go-openapi/swag/mangling` (v0.x) with an equivalent job and a
stricter, more robust, faster implementation. See [differences with v1](docs/v1-differences.md).

```go
import "github.com/go-openapi/swag/mangling/v2"
```

## The three manglers

**`Mangler`** — general-purpose recasing.

```go
m := mangling.MakeMangler()
m.Camelize("sample text")  // "sampleText"
m.Snakize("sampleText")    // "sample_text"
m.Titleize("hello world")  // "Hello World"
```

**`GoMangler`** — names a Go code generator can safely emit. Any input yields a valid,
non-empty identifier.

```go
g := mangling.MakeGoMangler()
g.IdentExported("sample text") // "SampleText"
g.ConstName("status 200")      // "StatusTwoHundred"
g.File("test.go")              // "test_swagger.go"
```

**`numbers.NumberMangler`** — verbalize numbers in text.

```go
nm := numbers.MakeNumberMangler()
nm.NumberWords("0.25") // "one quarter"
```

Manglers are immutable values, configured with functional options at construction
(`MakeXxx` returns a value, `NewXxx` a pointer), and safe for concurrent use.

## Documentation

- [Design](docs/design.md) — the pipeline, the token model, and the roadmap.
- [Go identifiers](docs/go-identifiers.md) — the "always a valid Go identifier" guarantee,
  `Ident*` / `ConstName` / `File` / `Package` / `Module`, and the repairs.
- [ASCII-fication](docs/asciification.md) — folding and romanization: `café → Cafe`,
  Cyrillic / Greek / Arabic, `½ → OneHalf`, `😀 → GrinningFace`, Japanese kana.
- [Numbers](docs/numbers.md) — cardinals, fractions, digit-group reconstruction.
- [Differences with v1](docs/v1-differences.md) — migration and rationale.

Full API reference: [pkg.go.dev](https://pkg.go.dev/github.com/go-openapi/swag/mangling/v2).

## Not supported

- CJK Han ideographs (Kanji / Hanzi) — elided (Japanese kana works); native-script identifiers
  remain available with folding off.
- Korean Hangul — elided, both syllables and standalone Jamo letters (a future improvement could
  romanize the Jamo by letter name).
- Grapheme clusters (flag emoji, ZWJ sequences).

## Performance

A typical mangling takes on the order of 1,000 ns. All methods scale linearly with the number of
tokens (~160 ns/token) and perform zero internal allocation beyond the returned string —
see [our performance analysis](docs/performance.md).
This corresponds roughly a x3 improvement on the *swag* version.

## Tests

The Go mangler is fuzzed with the objective of producing a valid Go identifier against all-weather
input. Run the suite with `go test ./...`.
