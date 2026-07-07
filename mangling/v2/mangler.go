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
// Emoji runes and unicode grapheme clusters are not supported at this moment.
type Mangler struct {
	Tokenizer
	options // options for plurals (possibly - future - language)
}

// MakeMangler builds a [Mangler] value with optional settings.
func MakeMangler(opts ...Option) Mangler {
	var m Mangler
	m.options = buildOptions(m.options, opts)
	m.Tokenizer.tokenOptions = m.options.tokenOptions

	return m
}

// NewMangler builds a [Mangler] pointer with optional settings.
func NewMangler(opts ...Option) *Mangler {
	m := MakeMangler(opts...)

	return &m
}

// TargetTransform is a compiled, immutable recipe describing how to render a segmented token stream: casing ×
// separator × affix × stages × repair.
//
// All fields are unexported; build custom targets with [MakeTargetTransform].
// The mangler supplies the data (dictionaries) that stages bind to at run time, so a target degrades gracefully across
// manglers.
//
// Fields are unexported; the assembly recipe is casing × separator × symbol-policy (affix, stages and repair land
// later — §4.5, §10).
type TargetTransform struct {
	firstCasing  wordCasing   // casing of the first emitted word (camelCase lowercases it)
	restCasing   wordCasing   // casing of subsequent words
	separator    string       // "", "_", "-", " ", "."
	symbolPolicy symbolPolicy // drop | verbalize | keep
}

// MakeTargetTransform builds a custom [TargetTransform].
func MakeTargetTransform(opts ...TargetOption) TargetTransform {
	var tr TargetTransform
	for _, apply := range opts {
		tr = apply(tr)
	}

	return tr
}

// TargetOption customizes a [TargetTransform].
type TargetOption func(TargetTransform) TargetTransform

// WithSeparator sets the output separator emitted between tokens.
func WithSeparator(sep string) TargetOption {
	return func(tr TargetTransform) TargetTransform {
		tr.separator = sep

		return tr
	}
}

// Preset targets.
//
// These return a fresh immutable value (they are functions, not variables, so a caller can never corrupt a shared
// preset).
// Named after the form they produce.
func TargetTitle() TargetTransform {
	return TargetTransform{firstCasing: casingTitle, restCasing: casingTitle, separator: " "}
}

func TargetSentence() TargetTransform {
	return TargetTransform{firstCasing: casingTitle, restCasing: casingLower, separator: " "}
}

func TargetSnake() TargetTransform {
	return TargetTransform{firstCasing: casingLower, restCasing: casingLower, separator: "_"}
}

func TargetKebab() TargetTransform {
	return TargetTransform{firstCasing: casingLower, restCasing: casingLower, separator: "-"}
}

func TargetCamel() TargetTransform {
	return TargetTransform{firstCasing: casingLower, restCasing: casingTitle}
}

func TargetPascal() TargetTransform {
	return TargetTransform{firstCasing: casingTitle, restCasing: casingTitle}
}

func TargetAllCaps() TargetTransform {
	return TargetTransform{firstCasing: casingUpper, restCasing: casingUpper, separator: "_"}
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

// asciifyInput is the string-level half of ASCII-fication, applied before segmentation when folding
// is enabled: it expands non-foldable runes to their phonetic name (§4.7.1 tier 4) so multi-word names
// re-segment. Diacritics and combining marks are left for the token-level foldASCII stage. This is a
// neutral Mangler capability — shared by every preset and by GoMangler's ident pipeline.
func (m Mangler) asciifyInput(str string) string {
	if !m.asciify {
		return str
	}

	return expandRuneNames(str)
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

// Pluralize a word (a sentence?) (english)
//
// wolf -> wolves wolves -> (unchanged) chopper -> choppers.
func (m Mangler) Pluralize(string) string {
	return ""
}

// Singularize a word (english), the inverse of [Mangler.Pluralize].
//
// wolves -> wolf wolf -> (unchanged)
func (m Mangler) Singularize(string) string {
	return ""
}

/*
// Fix 3rd person conjugate, e.g. XYZ paint => XYZ paints.
// but: XYZs paint => invariant (plural)
// Do we really need that?
func (m Mangler) Conjugate(string) string {
	return ""
}
*/
