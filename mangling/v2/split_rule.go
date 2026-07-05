package mangling

// splitRule decides whether a token boundary falls at a given rune during segmentation.
//
// It is deliberately unexported: for this iteration the §4.2 boundary signals are opinionated and
// owned by the ruleset, not a user-pluggable knob. We keep the type here in isolation so we can
// later decide whether to promote it to public API (and settle a signature that carries enough
// context — lookback/lookahead) or drop it.
type splitRule func(rune, []byte) bool

func splitRuleCaseAlternance(r rune) bool {
	return false // TODO
}

func splitRuleSeparators(...rune) splitRule {
	return nil // TODO
}

func splitRuleDigitGroups(r rune) bool {
	return false // TODO
}
