package mangling

// Kind classifies a token produced by segmentation.
//
// The tokenizer emits [KindWord], [KindNumber] and [KindSymbol]; [KindInitialism] is set later by
// the initialism overlay (§4.4), never by the tokenizer.
type Kind uint8

const (
	KindWord       Kind = iota // a run of letters
	KindNumber                 // a run of decimal digits (Nd)
	KindSymbol                 // a single non-letter, non-digit, non-separator rune (@, #, …)
	KindInitialism             // retagged by the initialism overlay (HTTP, JSON, …)
)

// Casing describes the case pattern of a token, computed during segmentation.
type Casing uint8

const (
	CasingLower Casing = iota // lowercase run: "http"
	CasingUpper               // uppercase run (screaming / all-caps): "HTTP"
	CasingTitle               // title case: "Http"
	CasingMixed               // anything else ("hTtP"), or content with no case
)

// token is a zero-copy view into the shared []rune of a [Tokens] value: a half-open span plus the
// classification computed by the scanner.
//
// It is internal; transforms reach token data only through [Tokens]' index-based methods, so the
// struct can evolve (e.g. the override vs. side-arena question, §9) without touching the public API.
type token struct {
	start, end int    // half-open span [start,end) into Tokens.runes
	kind       Kind   // word | number | symbol | initialism
	casing     Casing // lower | upper | title | mixed
	override   string // rewritten content; empty unless a transform replaced the span
}
