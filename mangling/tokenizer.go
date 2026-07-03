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
