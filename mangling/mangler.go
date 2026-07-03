package mangling

// Mangler exposes general purpose well known case formatters that use simple rules.
//
// It works well with latin letters.
//
// non-letters remain unchanged. letters in languages that do not support case remain unchanged.
type Mangler struct {
	Tokenizer
	options // options for plurals (possibly - future - language)
}

// Titleize transforms all words in titled case.
//
// Like so:
//
//	This is Is A Title.
func (m Mangler) Titleize(string) string {
	return ""
}

// Humanize produces a space-sepatated sentence, first letter titleized, the rest lower-cased.
func (m Mangler) Humanize(string) string {
	return ""
}

// Snake case (lower-cased).
//
// Like so:
//
//	snake_case.
func (m Mangler) Snakize(string) string {
	return ""
}

// Kebab case (lower-cased)
//
// Like so:
//
//	bab-case
func (m Mangler) Kebabize(string) string {
	return ""
}

// Camel case, e.g. allCaps
func (m Mangler) Camelize(string) string {
	return ""
}

// All caps snakized (e.g ALL_CAPS)
func (m Mangler) Capitalize(string) string {
	return ""
}

// spew written numerals (english) ("123" => one hundred and twenty three)
// non numerals are kept as-is.
// 0.1 => one tenth
func (m Mangler) NumberWords(string) string {
	return ""
}

// Pluralize a word (a sentence?) (english)
//
// wolf -> wolves
// wolves -> (unchanged)
// chopper -> choppers
func (m Mangler) Pluralize(string) string {
	return ""
}

// Fix 3rd person conjugate, e.g. XYZ paint => XYZ paints.
// but: XYZs paint => invariant (plural)
// Do we really need that?
func (m Mangler) Conjugate(string) string {
	return ""
}
