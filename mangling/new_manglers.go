package mangling

import "iter"

// Tokenizer tokenize a string along separation rules and applies
// registered transforms to the tokens.
type Tokenizer struct {
	tokenOptions
}

func (m Tokenizer) Tokenize(string) iter.Seq[string] {
	return nil
}

func (m Tokenizer) ApplyTransforms(string) iter.Seq[string] {
	return nil
}

type (
	TokenOption  func(*tokenOptions)
	tokenOptions struct{}
)

func WithTransformers(...Transformer) TokenOption {
	return nil
}

func WithSeparators(...rune) TokenOption {
	return nil
}

func WithSplitRules(...SplitRule) TokenOption {
	return nil
}

type SplitRule func(r rune, buf []byte) bool

func SplitRuleCaseAlternance(r rune) bool {
	return false
}

func SplitRuleSeparators(...rune) SplitRule {
	return nil
}

type Transformer func(string) string

// common transforms
func Avoid(...string) Transformer           { return nil }
func Replace(map[string]string) Transformer { return nil }

// Mangler exposes general purpose well known case formatters that use simple rules.
//
// It works well with latin letters.
//
// non-letters remain unchanged. letters in languages that do not support case remain unchanged.
type Mangler struct {
	Tokenizer
	options // options for plurals (possibly - future - language)
}

// All words are titled
func (m Mangler) Titleize(string) string {
	return ""
}

// Decased, first letter titleized
func (m Mangler) Humanize(string) string {
	return ""
}

// Snake case (lower-cased)
func (m Mangler) Snakize(string) string {
	return ""
}

// Kebab case (lower-cased)
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
func (m Mangler) Conjugate(string) string {
	return ""
}

// GoMangle specializes in getting identifiers right for go conventions.
type GoMangler struct {
	Mangler
	goOptions
}

type goOptions struct{}

type GoOption func(*goOptions)

func WithGoDefaults() GoOption {
	return nil
}

func AddInitialisms(...string) GoOption {
	return nil
}

func UseInitialisms(...string) GoOption {
	return nil
}

// set plural forms for initialisms
func WithInitialismPlurals(...string) GoOption {
	return nil
}

func WithPrefixReplaceRules(...ReplaceRule) GoOption {
	return nil
}

type ReplaceRule uint8

const (
	ReplaceRulePrefix ReplaceRule = iota
	ReplaceRuleRuneName
	ReplaceRuleStrip
	// ...
)

func (g GoMangler) IdentUnexported(string) string {
	return ""
}

// exported identifier
func (g GoMangler) IdentExported(string) string {
	return ""
}

// legit package name
func (g GoMangler) Package(string) (alias string, pkg string) {
	return "", ""
}

// legit module name
func (g GoMangler) Module(string) string {
	return ""
}

// valid go file, taking file suffix conventions
// would be lower-cased, snakized
func (g GoMangler) File(string) string {
	return ""
}

// utiility functions (separate package?)

func NumberWords[T int | float32](n T) string { // TODO: larger class of type constraint covering all numerics -- see swag/conv formatting functions}
	return ""
}

// 31 -> 31st
func NumberOrdinal[T int | uint](n T) string { // TODO: larger class of type constraint covering all numerics -- see swag/conv formatting functions}
	return ""
}

// 4 -> iv
func NumberRoman[T int | uint](n T) string { // TODO: larger class of type constraint covering all numerics -- see swag/conv formatting functions}
	return ""
}

// é -> e (roman family)
func ToAscii(string) string { // TODO: larger class of type constraint covering string, []byte, []rune
	return ""
}

func UnicodeName[T rune | byte](r T) string { return "" }

// ValueMangler mangles arbitrary values into readable strings, using relaxed rules
//
// Has extended transliteration rules, e.g. for "+", "#" symbols.
//
// A mangled value can be thereafter made a Go identifier by call GoMangler.Ident().
type ValueMangler struct {
	Tokenizer
	valueOptions
}

type valueOptions struct{}
