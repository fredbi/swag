package mangling

// asciiFold maps a Latin letter bearing a diacritic (or a distinct Latin letter such as æ, ß, þ)
// to its plain ASCII equivalent, preserving case. It is the data behind [Ascii] / [ToAscii].
//
// Scope: European Latin scripts (Latin-1 Supplement, Latin Extended-A, a few Extended-B). This is
// diacritic *folding* — strip the accent, keep the base letter (ü→u, not the German ü→ue
// transliteration). Distinct letters that have no single-rune ASCII base fold to their conventional
// digraph (æ→ae, œ→oe, ß→ss, þ→th, ð→d).
//
// NOT covered here (by design): symbols and punctuation — see [defaultSymbolWords]; and non-Latin
// scripts (Greek, Cyrillic, CJK, …), which fall back to the phonetic rune name (see [UnicodeName]).
//
// NOTE: prepared as data only; not yet wired into the pipeline.
var asciiFold = map[rune]string{
	// A
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a", 'ǎ': "a",
	'À': "A", 'Á': "A", 'Â': "A", 'Ã': "A", 'Ä': "A", 'Å': "A", 'Ā': "A", 'Ă': "A", 'Ą': "A", 'Ǎ': "A",
	// AE (ligature / distinct letter)
	'æ': "ae", 'Æ': "AE",
	// C
	'ç': "c", 'ć': "c", 'ĉ': "c", 'ċ': "c", 'č': "c",
	'Ç': "C", 'Ć': "C", 'Ĉ': "C", 'Ċ': "C", 'Č': "C",
	// D (incl. đ d-bar and ð eth)
	'ď': "d", 'đ': "d", 'ð': "d",
	'Ď': "D", 'Đ': "D", 'Ð': "D",
	// E (incl. ə schwa)
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ĕ': "e", 'ė': "e", 'ę': "e", 'ě': "e", 'ə': "e",
	'È': "E", 'É': "E", 'Ê': "E", 'Ë': "E", 'Ē': "E", 'Ĕ': "E", 'Ė': "E", 'Ę': "E", 'Ě': "E",
	// G
	'ĝ': "g", 'ğ': "g", 'ġ': "g", 'ģ': "g",
	'Ĝ': "G", 'Ğ': "G", 'Ġ': "G", 'Ģ': "G",
	// H
	'ĥ': "h", 'ħ': "h",
	'Ĥ': "H", 'Ħ': "H",
	// I (incl. Turkish ı dotless and İ dotted)
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ĩ': "i", 'ī': "i", 'ĭ': "i", 'į': "i", 'ı': "i",
	'Ì': "I", 'Í': "I", 'Î': "I", 'Ï': "I", 'Ĩ': "I", 'Ī': "I", 'Ĭ': "I", 'Į': "I", 'İ': "I",
	// J
	'ĵ': "j", 'Ĵ': "J",
	// K
	'ķ': "k", 'Ķ': "K",
	// L (incl. ł l-stroke)
	'ĺ': "l", 'ļ': "l", 'ľ': "l", 'ŀ': "l", 'ł': "l",
	'Ĺ': "L", 'Ļ': "L", 'Ľ': "L", 'Ŀ': "L", 'Ł': "L",
	// N (incl. ŋ eng)
	'ñ': "n", 'ń': "n", 'ņ': "n", 'ň': "n", 'ŋ': "n",
	'Ñ': "N", 'Ń': "N", 'Ņ': "N", 'Ň': "N", 'Ŋ': "N",
	// O (incl. ø o-slash)
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ŏ': "o", 'ő': "o",
	'Ò': "O", 'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O", 'Ø': "O", 'Ō': "O", 'Ŏ': "O", 'Ő': "O",
	// OE (ligature)
	'œ': "oe", 'Œ': "OE",
	// R
	'ŕ': "r", 'ŗ': "r", 'ř': "r",
	'Ŕ': "R", 'Ŗ': "R", 'Ř': "R",
	// S (incl. ș s-comma)
	'ś': "s", 'ŝ': "s", 'ş': "s", 'š': "s", 'ș': "s",
	'Ś': "S", 'Ŝ': "S", 'Ş': "S", 'Š': "S", 'Ș': "S",
	// SS (sharp s)
	'ß': "ss", 'ẞ': "SS",
	// T (incl. ț t-comma)
	'ţ': "t", 'ť': "t", 'ŧ': "t", 'ț': "t",
	'Ţ': "T", 'Ť': "T", 'Ŧ': "T", 'Ț': "T",
	// TH (thorn)
	'þ': "th", 'Þ': "Th",
	// U
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ũ': "u", 'ū': "u", 'ŭ': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'Ù': "U", 'Ú': "U", 'Û': "U", 'Ü': "U", 'Ũ': "U", 'Ū': "U", 'Ŭ': "U", 'Ů': "U", 'Ű': "U", 'Ų': "U",
	// W
	'ŵ': "w", 'Ŵ': "W",
	// Y
	'ý': "y", 'ÿ': "y", 'ŷ': "y",
	'Ý': "Y", 'Ÿ': "Y", 'Ŷ': "Y",
	// Z
	'ź': "z", 'ż': "z", 'ž': "z",
	'Ź': "Z", 'Ż': "Z", 'Ž': "Z",
}
