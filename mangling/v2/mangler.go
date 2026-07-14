// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"unicode/utf8"

	"github.com/go-openapi/swag/mangling/v2/internal/tokens"
	"github.com/go-openapi/swag/mangling/v2/numbers"
)

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
	tokens.Tokenizer
	options // options for plurals (possibly - future - language)

	// num verbalizes numeral runes in the asciify pass (½ → "one half"). The base Mangler carries a default
	// NumberMangler; the GoMangler overrides it with its configured one so a numeral rune and the same value written as
	// digits verbalize consistently.
	num numbers.NumberMangler
}

// MakeMangler builds a [Mangler] value with optional settings.
func MakeMangler(opts ...Option) Mangler {
	var m Mangler
	m.options = buildOptions(m.options, opts)
	m.Separator = m.separator // Tokenizer.Separator (public) <- the resolved option (private, defaulted in buildOptions)
	m.num = numbers.MakeNumberMangler()

	return m
}

// NewMangler builds a [Mangler] pointer with optional settings.
func NewMangler(opts ...Option) *Mangler {
	m := MakeMangler(opts...)

	return &m
}

// Transform renders str through the target recipe: asciify the input (operator verbalization always, rune-name
// verbalization when folding is on) → segment → ASCII-fold the tokens when folding is on → assemble (casing ×
// separator × symbol policy).
//
// The [GoMangler] inserts an initialism overlay before assembly; the base [Mangler] has no initialism stage.
func (m Mangler) Transform(target TargetTransform, str string) string {
	str = m.asciifyInput(str)

	t := tokens.Borrow(str)
	defer t.Redeem()

	m.Segment(&t)

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
// re-case per word: operator verbalization ([expandOperators], always) and rune-name asciification ([expandRuneNames],
// when folding is enabled).
//
// A single byte scan decides which passes are needed, so the fast path (pure ASCII with no operator lead byte) walks
// the input once and returns it untouched — neither pre-pass runs and neither re-scans. A glyph operator (≠, ≤, →) is
// non-ASCII but its UTF-8 lead byte (0xE2/0xC2) is an operator lead, so it still flags hasOperator.
//
// Diacritics and combining marks are left for the token-level foldASCII stage; this is a neutral Mangler capability.
func (m Mangler) asciifyInput(str string) string {
	var hasOperator, hasNonASCII bool

	for i := 0; i < len(str); i++ {
		c := str[i]
		if c >= utf8.RuneSelf {
			hasNonASCII = true
		}

		if operatorLeadByte[c] {
			hasOperator = true
		}

		if hasOperator && hasNonASCII {
			break
		}
	}

	if hasOperator {
		str = expandOperators(str)
	}

	if m.asciify && hasNonASCII {
		str = expandRuneNames(str, m.num)
	}

	return str
}
