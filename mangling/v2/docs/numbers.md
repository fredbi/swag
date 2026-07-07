# Numbers

The `numbers` sub-package verbalizes numbers found in text. It is a standalone engine: unlike the
name-oriented manglers it does its own number-aware scanning (it needs to see decimal points and
digit-group separators that the general tokenizer would drop), so it stands on its own.

```go
import "github.com/go-openapi/swag/mangling/v2/numbers"

nm := numbers.MakeNumberMangler()
nm.NumberWords("42")   // "forty two"
nm.NumberWords("1024") // "one thousand and twenty four"
```

`AppendWords(dst, in)` is the allocation-free variant that appends into a caller-owned buffer.

## What it recognizes

Numbers are verbalized wherever they appear in the input; surrounding words are passed through.

| Input | Output |
|---|---|
| `0` | `zero` |
| `300` | `three hundred` |
| `-5` | `minus five` |
| `3.14` | `three dot one four` |
| `order 1234 shipped` | `order one thousand two hundred and thirty four shipped` |
| `1,234` | `one thousand two hundred and thirty four` (digit groups reconstructed) |

Integers too large for `int64` are spelled digit by digit, so no input ever overflows:

```go
nm.NumberWords("9999999999999999999") // "nine nine nine ... nine"
```

## Fractions

Common decimal values are recognized as fractions — the leap over inflect-era codegen, which
could only spell digits:

| Input | Output |
|---|---|
| `0.5` | `one half` |
| `0.25` | `one quarter` |
| `0.75` | `three quarters` |
| `0.1` | `one tenth` |
| `0.333` | `one third` |

Matching is fuzzy within a detection precision, so `0.333` and `0.3333333` both resolve to
`one third`. Tune it with `WithNumberDetectPrecision` when you need to distinguish closer values.

## Options

```go
nm := numbers.MakeNumberMangler(
    numbers.WithNumberStripAnd(true),
    numbers.WithSpecialNumbers(map[string]string{"3.1415": "pi"}),
)
```

| Option | Effect |
|---|---|
| `WithNumberStripAnd(true)` | drop the connective "and": `1024` → `one thousand twenty four` |
| `WithNumberStripOne(true)` | drop a leading "one": `100` → `hundred` |
| `WithSpecialNumbers(map)` | register named constants matched ahead of cardinal/fraction rendering: `3.1415` → `pi` |
| `WithNumberDetectPrecision(n)` | decimal places used when matching fractions and special numbers |

## Numeral runes

`RuneNumber` reports the numeric value of a single Unicode numeral rune — the No (vulgar
fractions, circled digits, ...) and Nl (roman, ...) categories:

```go
numbers.RuneNumber('½') // 0.5, true
numbers.RuneNumber('Ⅶ') // 7,   true
numbers.RuneNumber('①') // 1,   true
```

Decimal digits (category Nd, including non-ASCII ones like `٧`) are handled directly by the
manglers through a digit-value offset, so they are not part of this map. See
[asciification.md](asciification.md) for how numeral runes surface in identifiers.

## From the `GoMangler`

`GoMangler.ConstName` runs its numbers through this engine, and `WithGoNumberOptions` forwards
number options to it:

```go
g := mangling.MakeGoMangler(
    mangling.WithGoNumberOptions(
        numbers.WithSpecialNumbers(map[string]string{"3.1415": "pi"}),
    ),
)
g.ConstName("3.1415") // "Pi"
g.ConstName("0.25")   // "OneQuarter"
```
