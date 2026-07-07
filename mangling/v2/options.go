package mangling

import "github.com/go-openapi/swag/mangling/v2/numbers"

type (
	// TokenOption customizes the behavior of the [Tokenizer].
	TokenOption func(tokenOptions) tokenOptions

	// Option customizes the behavior of the [Mangler].
	Option func(options) options

	// GoOption customizes the behavior of the [GoMangler].
	GoOption func(goOptions) goOptions

	// ValueOption customizes value verbalization (e.g. [GoMangler.ConstName]).
	ValueOption func(valueOptions) valueOptions
)

type (
	tokenOptions struct {
		separator func(rune) bool // separator identities used to split tokens
		rules     []splitRule
	}

	options struct {
		tokenOptions
		asciify bool // fold Latin diacritics to ASCII (off in base Mangler, on in GoMangler)
	}

	goOptions struct {
		options

		initialisms      []string
		keywords         map[string]struct{}
		builtins         map[string]struct{}
		fileSuffixes     map[string]struct{}
		reservedSuffix   string                 // appended to an ident colliding with a reserved word (default "Var")
		fileRepairSuffix string                 // appended to a file stem ending in a GOOS/GOARCH/test suffix (default "swagger")
		numberOpts       []numbers.NumberOption // configure the NumberMangler used by ConstName / leading-digit verbalization
	}

	valueOptions struct{}
)

func buildTokenOptions(o tokenOptions, opts []TokenOption) tokenOptions {
	for _, apply := range opts {
		o = apply(o)
	}

	// defaults
	if o.separator == nil {
		o.separator = defaultTokenSeparator
	}

	return o
}

func buildOptions(o options, opts []Option) options {
	for _, apply := range opts {
		o = apply(o)
	}

	return o
}

func buildGoOptions(o goOptions, opts []GoOption) goOptions {
	o.options.asciify = true // GoMangler default: fold to ASCII (gosmopolitan-clean); opts may override

	for _, apply := range opts {
		o = apply(o)
	}

	// defaults: the Go ruleset dictionaries, as detection sets
	if o.initialisms == nil {
		o.initialisms = DefaultInitialisms()
	}
	if o.keywords == nil {
		o.keywords = goKeywordsSet
	}
	if o.builtins == nil {
		o.builtins = goBuiltinsSet
	}
	if o.fileSuffixes == nil {
		o.fileSuffixes = goFileSuffixesSet
	}
	if o.reservedSuffix == "" {
		o.reservedSuffix = "Var" // go-swagger's convention: "type" -> "typeVar"
	}
	if o.fileRepairSuffix == "" {
		o.fileRepairSuffix = "swagger" // "test.go" -> "test_swagger.go"
	}

	return o
}

// toSet turns a list of words into a detection set.
func toSet(list []string) map[string]struct{} {
	set := make(map[string]struct{}, len(list))
	for _, item := range list {
		set[item] = struct{}{}
	}

	return set
}

func WithTokenSeparator(separator func(rune) bool) TokenOption {
	return func(o tokenOptions) tokenOptions {
		o.separator = separator

		return o
	}
}

func WithSeparators(...rune) TokenOption {
	return nil
}

func WithTokenOptions(opts ...TokenOption) Option {
	return func(o options) options {
		o.tokenOptions = buildTokenOptions(o.tokenOptions, opts)

		return o
	}
}

// WithAsciiFolding toggles folding of Latin diacritics to ASCII (é→e, ñ→n, ß→ss, combining marks stripped).
//
// It is off by default in the base [Mangler] and on by default in the [GoMangler] (gosmopolitan-clean output).
// Non-Latin scripts (CJK, …) are left as-is — a future rune-name concern.
func WithAsciiFolding(enabled bool) Option {
	return func(o options) options {
		o.asciify = enabled

		return o
	}
}

func WithManglerOptions(opts ...Option) GoOption {
	return func(o goOptions) goOptions {
		o.options = buildOptions(o.options, opts)

		return o
	}
}

// WithGoNumberOptions configures the [numbers.NumberMangler] the [GoMangler] uses to verbalize numbers in
// [GoMangler.ConstName] and leading-digit identifiers — e.g. registering special numbers or eliding "and"/"one".
func WithGoNumberOptions(opts ...numbers.NumberOption) GoOption {
	return func(o goOptions) goOptions {
		o.numberOpts = append(o.numberOpts, opts...)

		return o
	}
}

func WithGoDefaults() GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

func WithGoInitialisms(...string) GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

func UseGoInitialisms(...string) GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

// set plural forms for initialisms
func WithGoInitialismPlurals(...string) GoOption {
	return func(o goOptions) goOptions {
		return o
	}
}

var goReservedWords = []string{
	"break",
	"case",
	"chan",
	"const",
	"continue",
	"default",
	"defer",
	"else",
	"fallthrough",
	"for",
	"func",
	"go",
	"goto",
	"if",
	"import",
	"interface",
	"map",
	"package",
	"range",
	"return",
	"select",
	"struct",
	"switch",
	"type",
	"var",
}

var goBuiltins = []string{
	"append",
	"print",
	"cap",
	"clear",
	"close",
	"complex",
	"copy",
	"delete",
	"len",
	"make",
	"max",
	"min",
	"new",
	"panic",
	"println",
	"real",
	"recover",
}

var goFileSuffixes = []string{
	// goos
	"aix",
	"android",
	"darwin",
	"dragonfly",
	"freebsd",
	"hurd",
	"illumos",
	"ios",
	"js",
	"linux",
	"nacl",
	"netbsd",
	"openbsd",
	"plan9",
	"solaris",
	"windows",
	"zos",

	// arch
	"386",
	"amd64",
	"amd64p32",
	"arm",
	"armbe",
	"arm64",
	"arm64be",
	"loong64",
	"mips",
	"mipsle",
	"mips64",
	"mips64le",
	"mips64p32",
	"mips64p32le",
	"ppc",
	"ppc64",
	"ppc64le",
	"riscv",
	"riscv64",
	"s390",
	"s390x",
	"sparc",
	"sparc64",
	"wasm",

	// other reserved suffixes
	"test",
}

// Precomputed default detection sets for the Go ruleset (shared, read-only after init).
var (
	goKeywordsSet     = toSet(goReservedWords)
	goBuiltinsSet     = toSet(goBuiltins)
	goFileSuffixesSet = toSet(goFileSuffixes)
)

func DefaultInitialisms() []string {
	return []string{
		"ACL",
		"API",
		"ASCII",
		"CPU",
		"CSS",
		"DNS",
		"EOF",
		"GUID",
		"HTML",
		"HTTPS",
		"HTTP",
		"ID",
		"IP",
		"IPv4", // prefer the mixed case outcome IPv4 over the capitalized IPV4
		"IPv6", // prefer the mixed case outcome IPv6 over the capitalized IPV6
		"JSON",
		"LHS",
		"OAI",
		"QPS",
		"RAM",
		"RHS",
		"RPC",
		"SLA",
		"SMTP",
		"SQL",
		"SSH",
		"TCP",
		"TLS",
		"TTL",
		"UDP",
		"UI",
		"UID",
		"UUID",
		"URI",
		"URL",
		"UTF8",
		"VM",
		"XML",
		"XMPP",
		"XSRF",
		"XSS",
	}
}
