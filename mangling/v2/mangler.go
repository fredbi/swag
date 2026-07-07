package mangling

// Mangler exposes general purpose well-known case formatters with simple recasing rules.
//
// It works best with latin letters and unicode letters that define upper and lowercase classes.
//
// # Case handling
//
// Casing follows unicode rules: upper casing uses unicode "title-case".
//
// Special casing rules (e.g. [unicode.SpecialCase]) are not supported at this moment.
//
// Letters in languages that do not support case remain unchanged.
//
// # Symbols verbalization
//
// Common symbols such as "?", "@", "#" are verbalized and replaced by a short word (e.g. "question", "at", "hash").
//
// # ASCII transform
//
// By default, all letters and digits remain unchanged.
//
// The mangler may optionally ASCII-fy letters: latin letters with diacritics (e.g. é, ü) and unicode digits are
// converted to an ASCII equivalent, while non-latin unicode gets "phonetized" using its rune name.
//
// # NOTES
//
// CJK runes are elided, as no easy phonetization scheme is available.
//
// Unicode grapheme clusters are not supported at this moment.
//
// ASCII and numerals:
//
//  -  ASCII dDigits are left as-is
//  -  a "." (dot) is verbalized as "dot" (symbol), a "," comma is elided (separator)
//  - unicode numerals, such as ½, are represented numerically as "0.5" verbalized as "0dot5"

type Mangler struct {
	tokenizer
	options // options for plurals (possibly - future - language)
}

// MakeMangler builds a [Mangler] value with optional settings.
func MakeMangler(opts ...Option) Mangler {
	var m Mangler
	m.options = buildOptions(m.options, opts)
	m.tokenizer.tokenOptions = m.options.tokenOptions

	return m
}

// NewMangler builds a [Mangler] pointer with optional settings.
func NewMangler(opts ...Option) *Mangler {
	m := MakeMangler(opts...)

	return &m
}

// Transform renders str through the target recipe: segment → assemble (casing × separator × symbol policy).
//
// Stages (verbalization, folding, initialisms) will run between the two.
func (m Mangler) Transform(target TargetTransform, str string) string {
	str = m.asciifyInput(str)

	t := borrowTokens(str)
	defer t.redeem()

	m.segment(&t)

	if m.asciify {
		m.foldASCII(&t)
	}

	return m.assemble(&t, target)
}

// Titleize transforms all words in titled case.
//
// The remainder of each word is lower-cased.
//
// Like so:
//
//	This Is A Title.
func (m Mangler) Titleize(str string) string {
	return m.Transform(TargetTitle(), str)
}

// Humanize produces a space-separated sentence, first letter titleized, the rest lower-cased.
func (m Mangler) Humanize(str string) string {
	return m.Transform(TargetSentence(), str)
}

// Snakize produces snake case (lower-cased).
//
// Like so:
//
//	snake_case.
func (m Mangler) Snakize(str string) string {
	return m.Transform(TargetSnake(), str)
}

// Kebabize produces kebab case (lower-cased)
//
// Like so:
//
//	kebab-case.
func (m Mangler) Kebabize(str string) string {
	return m.Transform(TargetKebab(), str)
}

// Camelize produces camel case.
//
// Like so:
//
//	camelCase
func (m Mangler) Camelize(str string) string {
	return m.Transform(TargetCamel(), str)
}

// Pascalize produces pascal case.
//
// Like so:
//
//	PascalCase
func (m Mangler) Pascalize(str string) string {
	return m.Transform(TargetPascal(), str)
}

// AllCaps produces all letters capitalized and words snakized.
//
// Like so:
//
//	ALL_CAPS
func (m Mangler) AllCaps(str string) string {
	return m.Transform(TargetAllCaps(), str)
}

// Pluralization (Pluralize/Singularize) is deliberately NOT part of the mangler API: it is orthogonal to
// tokenization (it inflects a single word), so it will ship as a standalone `plurals` subpackage
// (English rules + irregular/uncountable tables) — see DESIGN_v2.md §13 (P2). The former empty stubs were
// removed to avoid shipping silent-empty methods on the public surface.

// asciifyInput is the string-level half of ASCII-fication, applied before segmentation when folding
// is enabled: it expands non-foldable runes to their phonetic name so multi-word names re-segment.
//
// Diacritics and combining marks are left for the token-level foldASCII stage.
//
// This is a neutral Mangler capability — shared by every preset and by GoMangler's ident pipeline.
func (m Mangler) asciifyInput(str string) string {
	if !m.asciify {
		return str
	}

	return expandRuneNames(str)
}
