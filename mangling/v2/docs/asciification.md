# ASCII-fication

ASCII-fication turns letters and symbols outside the ASCII range into an ASCII equivalent, so the
result passes linters like `gosmopolitan` and `asciicheck` and reads cleanly in generated code.

It is the biggest single leap over v1, which had no folding stage and would let Unicode leak
straight into identifiers.

Folding is **off by default** in the base `Mangler` and **on by default** in the `GoMangler`.
Toggle it with `WithASCIIFolding`:

```go
m := mangling.MakeMangler(mangling.WithASCIIFolding(true))
g := mangling.MakeGoMangler() // folding already on
```

## The tiers

ASCII-fication is applied in tiers, from cheapest to most approximate.

### 1. Latin diacritics fold to their base letter

```go
g.ConstName("café résumé")  // "CafeResume"
g.File("café résumé")       // "cafe_resume"
```

`é → e`, `ñ → n`, `ß → ss`; combining marks are stripped. This is a true fold — the letter is
recognizably preserved.

### 2. Non-Latin letters romanize by Unicode rune name

Scripts that have no Latin fold are romanized letter-by-letter using each rune's Unicode name.
This is deterministic and always ASCII-safe. It is **not** a linguistic transliteration — a rune
becomes the *name* Unicode gives it, not its sound.

For **Greek** this reads cleanly, because the letter names are words we already use:

| Input | `ConstName` |
|---|---|
| `λ` | `Lambda` |
| `αβγ` | `AlphaBetaGamma` |
| `πΣΩ` | `PiSigmaOmega` |

(λ's Unicode name is actually "lamda"; we normalize that one to "lambda".)

For most other scripts you get the letter *names*, which spell the word out rather than transliterate
it — stable and unique, but not pretty:

| Input | `ConstName` | letter names |
|---|---|---|
| `Иди` | `IDeI` | Cyrillic i-de-i — not "idi" |
| `Мир` | `EmIEr` | Cyrillic em-i-er — not "mir" |
| `مرحبا` | `MeemRehHahBehAlef` | Arabic meem-reh-hah-beh-alef |

The value proposition here is a *guaranteed, stable, ASCII* identifier — not a pretty one.

### 3. Numerals

Unicode numerals are recognized and rendered as words by the name manglers:

```go
g.ConstName("½")   // "OneHalf"    — vulgar fraction (category No)
g.ConstName("٧")   // "Seven"      — Arabic-Indic digit (category Nd)
g.ConstName("Ⅶ")   // "Seven"      — Roman numeral (category Nl)
g.ConstName("①")   // "One"        — circled digit
```

See [numbers.md](numbers.md) for the full number engine.

### 4. Emoji and other named symbols

```go
g.ConstName("😀")  // "GrinningFace"  — from the rune name "GRINNING FACE"
```

### 5. Even Japanese kana works

Hiragana and Katakana romanize through the rune-name table (a Kunrei/Nihon-shiki style: し→si,
ち→ti), so Japanese written in kana produces a readable romaji identifier out of the box:

```go
g.ConstName("カタカナ")   // "KaTaKaNa"
g.ConstName("こんにちは")  // "KoNNiTiHa"
```

## Not supported

- **CJK Han ideographs** (Kanji / Hanzi — the shared *CJK Unified Ideographs* block) are **elided**.
  There is no reliable character-level romanization (many characters are polyphonic), so rather than
  guess, they are dropped — and the empty-input fallback still guarantees a valid identifier:

  ```go
  g.ConstName("中文")  // "Empty"
  ```

  Note this affects Han specifically: Japanese *kana* (tier 5 above) romanize today. Native-script
  identifiers also remain available with folding turned off.

- **Hangul** (Korean) is **elided** too — both the composed syllables (`가`–`힣`, algorithmically named
  like the Han block) and the individual **Jamo** letters (`ㄱ`, `ㅏ`, …). A possible future improvement
  is to romanize the standalone Jamo by their letter name (`ㄱ → kiyeok`, `ㅏ → a`), the way Greek and
  Cyrillic letters already resolve; it is left out for now as standalone Jamo are rare in identifiers.

- **Grapheme clusters** (flag emoji, ZWJ sequences) are not grouped before folding. This is a
  planned enhancement, thin in value for identifiers.

## `ToASCII`: the plain-text folder

Alongside the manglers, the package-level `ToASCII` function folds a string without any casing or
tokenization. It differs from the manglers on numerals: it renders a numeral as a plain number
rather than as words.

```go
mangling.ToASCII("café")  // "cafe"
mangling.ToASCII("½")     // "0.5"   — numeric, where ConstName gives "OneHalf"
mangling.ToASCII("😀")    // "grinning face"
```

`RuneToASCII` does the same for a single rune, and `RuneShortName` returns a rune's short name.
