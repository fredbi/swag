package mangling

// Mangler exposes general purpose well-known case formatters ([Mangler.Camelize], [Mangler.Snakize],
// [Mangler.Kebabize], [Mangler.Titleize], ...) with simple recasing rules.
//
// It works best with latin letters and unicode letters that define upper and lowercase classes. ASCII folding is
// off by default (see [GoMangler] for the on-by-default variant).
//
// Case handling, symbol verbalization, ASCII folding and numeral handling are shared with [GoMangler] and
// documented in the package overview — see the "Case handling", "Symbol verbalization", "ASCII folding" and
// "Numerals" sections there.
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
//	snake_case
func (m Mangler) Snakize(str string) string {
	return m.Transform(TargetSnake(), str)
}

// Kebabize produces kebab case (lower-cased)
//
// Like so:
//
//	kebab-case
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

// Pluralization (Pluralize/Singularize) is deliberately NOT part of the mangler API: it is orthogonal to tokenization
// — it inflects a single word — so it belongs in a standalone helper (English rules plus an irregular/uncountable
// table), not on the mangler.

// asciifyInput runs the string-level input expansions before segmentation, so multi-word replacements re-segment and
// re-case per word:
//
//   - operator verbalization ([expandOperators]), always — "!=" → "not equal" — since it is a symbol concern, not a
//     folding one;
//   - rune-name asciification ([expandRuneNames]), only when folding is enabled — non-foldable runes to their phonetic
//     name.
//
// Diacritics and combining marks are left for the token-level foldASCII stage.
//
// This is a neutral Mangler capability — shared by every preset and by GoMangler's ident pipeline.
func (m Mangler) asciifyInput(str string) string {
	str = expandOperators(str)

	if !m.asciify {
		return str
	}

	return expandRuneNames(str)
}
