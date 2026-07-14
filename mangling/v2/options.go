// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"maps"

	"github.com/go-openapi/swag/mangling/v2/numbers"
)

type (
	// TokenOption customizes the behavior of the tokenizer (see the internal tokens package).
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

		// symbolWords maps a symbol rune to the word it verbalizes to; nil means the built-in defaultSymbolWords. It is
		// dual-purpose: its keys drive segmentation (a rune in the set is a symbol token, not a separator) and its values
		// drive verbalization (the word the assembler emits). Customized via WithSymbolWords.
		symbolWords map[rune]string
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

	// The separator default is not resolved here: it depends on the symbol set (see resolveSymbolSeparator), which lives
	// on options and is only final once all options are applied. A nil separator is filled in by buildOptions /
	// buildGoOptions.
	return o
}

// resolveSymbolSeparator finalizes the symbol set and the separator predicate, once every option has been applied.
//
// The separator depends on the symbol set (the set's keys decide which runes are symbol tokens vs. separators), so it
// can only be built here. The uncustomized case reuses the shared, allocation-free defaultTokenSeparator; a customized
// set builds a per-mangler predicate (see separatorRule). A separator supplied wholesale via WithTokenSeparator wins
// and is left untouched.
func resolveSymbolSeparator(o options) options {
	custom := o.symbolWords != nil
	if !custom {
		o.symbolWords = defaultSymbolWords // shared, read-only: the assembler only reads it
	}
	if o.separator == nil {
		if custom {
			o.separator = newSeparatorRule(o.symbolWords).sep
		} else {
			o.separator = defaultTokenSeparator
		}
	}

	return o
}

func buildOptions(o options, opts []Option) options {
	for _, apply := range opts {
		o = apply(o)
	}

	return resolveSymbolSeparator(o)
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
	o.options = resolveSymbolSeparator(o.options) // symbol set + separator, same as the base Mangler

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

// WithSymbolWords customizes the symbol-verbalization set on top of the built-in defaults ([DefaultSymbolWords]).
//
// Each entry overlays the defaults: a new key adds a symbol, an existing key overrides its word, and an **empty word
// removes** the symbol from the set. The rest of the defaults are kept — call [DefaultSymbolWords] and build a fresh
// map if you need wholesale control.
//
// The set is dual-purpose, so an edit affects two stages:
//   - segmentation: a rune in the set is a single-rune *symbol token*; a rune not in the set falls to the category
//     rules and is typically a separator. So adding ',' makes "a,b" tokenize as [a , b] (→ verbalizable), while removing
//     '@' makes "a@b" tokenize as [a b] (the '@' becomes a separator, dropped).
//   - verbalization: under a target's verbalize policy, the word is what the assembler emits ("@" → "at").
//
// Repeated calls accumulate. Applies to the base [Mangler] and, via [WithManglerOptions], to the [GoMangler].
func WithSymbolWords(words map[rune]string) Option {
	return func(o options) options {
		if o.symbolWords == nil {
			o.symbolWords = maps.Clone(defaultSymbolWords) // copy-on-write: never mutate the shared default
		}
		for r, w := range words {
			if w == "" {
				delete(o.symbolWords, r) // empty word removes the symbol from the set

				continue
			}
			o.symbolWords[r] = w
		}

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
