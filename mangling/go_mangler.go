package mangling

// GoMangle specializes in getting identifiers right for go conventions.
type GoMangler struct {
	Mangler

	goOptions
}

type goOptions struct {
	indexOfInitialisms
	tokenSeparators []rune
}

type GoOption func(goOptions) goOptions

func WithGoDefaults() GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

func AddInitialisms(...string) GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

func UseInitialisms(...string) GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

// set plural forms for initialisms
func WithInitialismPlurals(...string) GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

func WithPrefixReplaceRules(...ReplaceRule) GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

type ReplaceRule uint8

const (
	// simple prefix replacement when the first rune is not a letter
	ReplaceRulePrefix ReplaceRule = iota
	// use unicode rune name
	ReplaceRuleRuneName
	// strip non-letter initial rune
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
// would be lower-cased, snakized. No ".go" suffix is added.
//
// Exclude meaningful names like:
// xx_test
// xx_linux
// xxx_go125 etc
func (g GoMangler) File(string) string {
	return ""
}

// utility functions (separate package?)

func ToAscii[T ~string | ~[]byte](T) string { // TODO: larger class of type constraint covering string, []byte, []rune
	return ""
}

func UnicodeName[T ~rune | ~byte](r T) string { return "" }

// ValueMangler mangles arbitrary values into readable strings, using relaxed rules ("verbalization").
//
// Has extended transliteration rules, e.g. for "+", "#" symbols.
//
// A mangled value can be thereafter made a Go identifier by call GoMangler.IdentExported().
type ValueMangler struct {
	Tokenizer

	valueOptions
}

type ValueOption func(valueOptions) valueOptions

type valueOptions struct{}

func (g ValueMangler) Verbalize(string) string {
	return ""
}
