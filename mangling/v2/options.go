package mangling

import "github.com/go-openapi/swag/mangling/v2/numbers"

type (
	// TokenOption customizes the behavior of the [tokenizer].
	TokenOption func(tokenOptions) tokenOptions

	// Option customizes the behavior of the [Mangler].
	Option func(options) options

	// GoOption customizes the behavior of the [GoMangler].
	GoOption func(goOptions) goOptions
)

type (
	tokenOptions struct {
		separator func(rune) bool // separator identities used to split tokens
	}

	options struct {
		tokenOptions

		asciify bool // fold Latin diacritics to ASCII (off in base Mangler, on in GoMangler)
	}

	goOptions struct {
		options

		initialisms      []string // base list; nil → DefaultInitialisms(). Replaced by UseGoInitialisms.
		extraInitialisms []string // appended on top of the base list by WithGoInitialisms
		keywords         map[string]struct{}
		builtins         map[string]struct{}
		fileSuffixes     map[string]struct{}
		reservedSuffix   string                 // appended to an ident colliding with a reserved word (default "Var")
		fileRepairSuffix string                 // appended to a file stem ending in a GOOS/GOARCH/test suffix (default "swagger")
		identFallback    string                 // word used when an identifier reduces to nothing (default "empty")
		numberOpts       []numbers.NumberOption // configure the NumberMangler used by ConstName / leading-digit verbalization
	}
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
	o.asciify = true // GoMangler default: fold to ASCII (gosmopolitan-clean); opts may override

	for _, apply := range opts {
		o = apply(o)
	}

	// defaults: the Go ruleset dictionaries, as detection sets
	if o.initialisms == nil {
		o.initialisms = DefaultInitialisms()
	}
	if len(o.extraInitialisms) > 0 {
		// combine into a fresh slice so we never mutate DefaultInitialisms' or the caller's backing array
		combined := make([]string, 0, len(o.initialisms)+len(o.extraInitialisms))
		combined = append(combined, o.initialisms...)
		combined = append(combined, o.extraInitialisms...)
		o.initialisms = combined
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
	if o.identFallback == "" {
		o.identFallback = defaultIdentFallback // "___" -> "Empty" / "empty" (cased per target)
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

// WithTokenSeparator sets the predicate that decides which runes split the input into tokens.
//
// The predicate reports whether a rune acts as a separator (dropped from the output). It replaces the
// default separator rule wholesale, so it must recognize every character to break on.
func WithTokenSeparator(separator func(rune) bool) TokenOption {
	return func(o tokenOptions) tokenOptions {
		o.separator = separator

		return o
	}
}

// WithTokenOptions bundles token-level options into a single mangler [Option].
//
// Use it to pass tokenizer settings (such as [WithTokenSeparator]) when configuring a [Mangler].
func WithTokenOptions(opts ...TokenOption) Option {
	return func(o options) options {
		o.tokenOptions = buildTokenOptions(o.tokenOptions, opts)

		return o
	}
}

// WithASCIIFolding toggles folding of Latin diacritics to ASCII (é→e, ñ→n, ß→ss, combining marks stripped).
//
// It is off by default in the base [Mangler] and on by default in the [GoMangler] (gosmopolitan-clean output).
// Most non-Latin scripts are romanized (with the notable exception of CJK runes, which are elided).
func WithASCIIFolding(enabled bool) Option {
	return func(o options) options {
		o.asciify = enabled

		return o
	}
}

// WithManglerOptions lifts base [Mangler] options into a [GoOption], so a [GoMangler] can be configured
// with the same settings as the base mangler it embeds (folding, token, initialism options, ...).
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

// WithGoIdentFallback sets the word substituted when an identifier reduces to nothing — i.e. the input is empty or
// made up entirely of separators / elided runes (e.g. "___", "@#$" with symbols dropped, or CJK under ASCII folding).
//
// Without it the Go identifier producers would emit an (invalid) empty string.
//
// The word is itself run through the mangler at the producing target, so any input is made valid and cased correctly:
// [GoMangler.IdentExported]/[GoMangler.ConstName] → "Empty", [GoMangler.IdentUnexported] → "empty",
// [GoMangler.File] → "empty".
//
// If the provided word *also* reduces to nothing, the built-in default "empty" is used, so a valid identifier is always
// produced.
// Applies to the [GoMangler] only; the base [Mangler] may still return "".
func WithGoIdentFallback(word string) GoOption {
	return func(o goOptions) goOptions {
		o.identFallback = word

		return o
	}
}

// WithGoReservedSuffix sets the suffix appended to an unexported identifier that collides with a Go keyword or builtin
// (default "Var": "type" → "typeVar", "append" → "appendVar").
//
// A house-style knob for generators that prefer a different convention.
func WithGoReservedSuffix(suffix string) GoOption {
	return func(o goOptions) goOptions {
		o.reservedSuffix = suffix

		return o
	}
}

// WithGoFileRepairSuffix sets the suffix appended to a file stem that would otherwise be build-constrained by a
// GOOS/GOARCH/`_test` suffix (default "swagger": "config_linux" → "config_linux_swagger").
//
// The go-swagger default is not appropriate for every generator, so it is configurable.
func WithGoFileRepairSuffix(suffix string) GoOption {
	return func(o goOptions) goOptions {
		o.fileRepairSuffix = suffix

		return o
	}
}

// WithGoInitialisms adds entries on top of the initialism list (the defaults, or the list set by [UseGoInitialisms]).
//
// Each string is the canonical casing to emit — e.g. "OAI", "gRPC"; matching is case-insensitive.
// Repeated calls accumulate.
func WithGoInitialisms(extra ...string) GoOption {
	return func(o goOptions) goOptions {
		o.extraInitialisms = append(o.extraInitialisms, extra...)

		return o
	}
}

// UseGoInitialisms replaces the default initialisms with the given list ([WithGoInitialisms] entries are still appended
// on top).
//
// Each string is the canonical casing to emit; matching is case-insensitive.
// Called with no arguments it is a no-op (the defaults stay).
func UseGoInitialisms(list ...string) GoOption {
	return func(o goOptions) goOptions {
		if len(list) > 0 {
			o.initialisms = list
		}

		return o
	}
}
