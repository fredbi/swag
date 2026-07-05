package mangling

// defaultSymbolWords maps a symbol rune to the word it verbalizes to (e.g. "@" => "at", "!" => "bang").
//
// This is the default data for the symbol policy of §4.7 (verbalization): when a target chooses to
// *verbalize* a symbol rather than drop it, this table supplies the word. It is deliberately narrow —
// only symbols that read meaningfully as a word.
//
// Explicitly NOT included (handled elsewhere, not by verbalization):
//   - separators and whitespace (space, and — depending on config — '-' '_' '.'): consumed by segmentation;
//   - structural/grouping punctuation (brackets, braces, parens, quotes): default policy drops them;
//   - letters with diacritics: folded to ASCII via [asciiFold].
//
// The current [defaultTokenSeparator] treats all [unicode.IsPunct] as a separator, which is too wide —
// it would elide the very symbols listed here before they could be verbalized. Reconciling that
// (separator set vs symbol-word set) is a wiring concern, deferred.
//
// NOTE: prepared as data only; not yet wired into the pipeline.
var defaultSymbolWords = map[rune]string{
	// operators & markers (ASCII)
	'@':  "at",
	'&':  "and",
	'#':  "hash",
	'%':  "percent",
	'+':  "plus",
	'=':  "equals",
	'*':  "star",
	'/':  "slash",
	'\\': "backslash",
	'|':  "pipe",
	'~':  "tilde",
	'^':  "caret",
	'!':  "bang",
	'?':  "question",
	'<':  "less",
	'>':  "greater",
	'$':  "dollar",
	'.':  "dot", // e.g. spelled decimals: "one dot two" (verbalize vs. elide is the target's symbol policy)

	// currency & misc symbols worth a word (non-ASCII)
	'€': "euro",
	'£': "pound",
	'¥': "yen",
	'¢': "cent",
	'©': "copyright",
	'®': "registered",
	'™': "trademark",
	'§': "section",
	'¶': "paragraph",
	'°': "degree",
	'µ': "micro",
	'×': "times",
	'÷': "divide",
	'±': "plusminus",
}
