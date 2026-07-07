package mangling

import (
	"maps"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/swag/mangling/v2/numbers"
	"github.com/go-openapi/swag/mangling/v2/runewords"
)

// GoMangler is a name mangler specialized in producing strings that abide by naming conventions used by go.
type GoMangler struct {
	Mangler
	n numbers.NumberMangler

	goOptions
	trie     *initialismTrie     // precomputed initialism index (shared, read-only)
	reserved map[string]struct{} // keywords ∪ builtins, for unexported-ident repair
}

// MakeGoMangler returns a value [GoMangler].
func MakeGoMangler(opts ...GoOption) GoMangler {
	var g GoMangler
	g.goOptions = buildGoOptions(g.goOptions, opts)
	g.Mangler.options = g.goOptions.options
	g.Mangler.Tokenizer.tokenOptions = g.goOptions.options.tokenOptions
	g.n = numbers.MakeNumberMangler(g.goOptions.numberOpts...)
	g.trie = buildInitialismTrie(g.goOptions.initialisms)

	g.reserved = make(map[string]struct{}, len(g.goOptions.keywords)+len(g.goOptions.builtins))
	maps.Copy(g.reserved, g.goOptions.keywords)
	maps.Copy(g.reserved, g.goOptions.builtins)

	return g
}

// repairReserved appends the reserved-word suffix (default "Var") when id collides with a Go keyword or builtin.
//
// Only unexported idents can collide — an exported (Title-cased) ident never equals a lowercase keyword/builtin, so
// it needs no repair.
func (g GoMangler) repairReserved(id string) string {
	if _, ok := g.reserved[id]; ok {
		return id + g.reservedSuffix
	}

	return id
}

// defaultIdentFallback is the built-in word used when an identifier reduces to nothing and the
// configured [WithGoIdentFallback] word (if any) also reduces to nothing. It must be a clean word that
// always survives mangling.
const defaultIdentFallback = "empty"

// orFallback guarantees a non-empty identifier: it returns id when non-empty, otherwise the configured
// fallback word mangled at the same target (so it is valid and cased to match — "Empty"/"empty"/snake),
// and finally the built-in [defaultIdentFallback] if even the configured word reduces to nothing.
//
// It is applied by the Go identifier producers (idents, const, file), never inside [GoMangler.identifier]
// itself — Package/Module intentionally allow an empty (dir-only) result.
func (g GoMangler) orFallback(id string, target TargetTransform) string {
	if id != "" {
		return id
	}
	if fb := g.identifier(g.identFallback, target); fb != "" {
		return fb
	}

	return g.identifier(defaultIdentFallback, target)
}

// identifier runs the Go ident pipeline: rune-name expansion → segment → ASCII fold → initialism
// overlay → assemble.
func (g GoMangler) identifier(str string, target TargetTransform) string {
	str = g.asciifyInput(str)

	t := borrowTokens(str)
	defer t.redeem()

	g.segment(&t)
	if g.Mangler.asciify {
		g.foldASCII(&t)
	}
	g.applyInitialisms(&t)

	return g.assemble(&t, target)
}

// expandRuneNames is the rune-name tier of asciification (§4.7.1 tier 4): every non-ASCII rune that
// diacritic folding won't handle (non-Latin letters, symbols, single-codepoint emoji) is replaced by
// its space-delimited phonetic name (π → " pi ", 😀 → " grinning face ") so it re-segments into words
// and re-cases per word (GrinningFace, not "Grinning face"). Runes the table elides (CJK ideographs,
// decorative symbols) are dropped. Foldable diacritics and combining marks pass through untouched for
// the token-level fold stage. Allocates only when a substitution or drop is actually needed.
func expandRuneNames(str string) string {
	need := false
	for _, r := range str {
		if r >= utf8.RuneSelf && !isCombiningMark(r) {
			if _, ok := asciiFold[r]; !ok {
				need = true

				break
			}
		}
	}
	if !need {
		return str // pure ASCII, or only diacritics/combining marks the fold stage handles
	}

	var b strings.Builder
	b.Grow(len(str) + 16)
	for _, r := range str {
		switch {
		case r < utf8.RuneSelf, isCombiningMark(r):
			b.WriteRune(r) // ASCII, or a combining mark left for the fold stage to strip
		default:
			if _, ok := asciiFold[r]; ok {
				b.WriteRune(r) // foldable diacritic: left for the fold stage
			} else if v, ok := numbers.RuneNumber(r); ok {
				b.WriteByte(' ')
				b.WriteString(formatNumeral(v)) // numeral rune → plain number ("½" → "0.5")
				b.WriteByte(' ')
			} else if w, ok := runewords.Word(r); ok {
				b.WriteByte(' ')
				b.WriteString(w)
				b.WriteByte(' ')
			} // else: an elided rune (CJK, decorative) — dropped
		}
	}

	return b.String()
}

// NewGoMangler returns a pointer to a [GoMangler].
func NewGoMangler(opts ...GoOption) *GoMangler {
	g := MakeGoMangler(opts...)

	return &g
}

// IdentUnexported produces a valid unexported go variable identifier from a string, possibly containing multiple words.
//
// IdentUnexported works like [Mangler.Camelize] with a few go-specific additions:
//   - compiler constraint: conflicts with go language keywords are resolved (e.g. "type" becomes "typeVar")
//   - compiler constraint: identifiers that don't start with a letter get their prefix verbalized (e.g. digits get
//     their numeral wording, symbols get replaced by a word, etc).
//   - linter constraint: conflicts with go builtin functions are resolved (e.g. "append" becomes "appendVar")
//
// An unexported identifier is camelized, with the casing of initialisms respected (e.g. "getHTTP" and not "getHttp").
func (g GoMangler) IdentUnexported(str string) string {
	return g.repairReserved(g.orFallback(g.identifier(g.verbalizeLeadingNumber(str), TargetCamel()), TargetCamel()))
}

// IdentExported produces a valid exported go variable identifier from a string, possibly containing multiple words.
//
// IdentExported works like [Mangler.Pascalize] with a few go-specific additions:
//   - compiler constraint: identifiers that don't start with a letter get their prefix verbalized (e.g. digits get
//     their numeral wording, symbols get replaced by a word, etc).
//
// Unlike their unexported counterpart, exported identifiers can't conflict with go reserved keywords or builtins.
func (g GoMangler) IdentExported(str string) string {
	return g.orFallback(g.identifier(g.verbalizeLeadingNumber(str), TargetPascal()), TargetPascal())
}

// Package produces a legit go package import path and its short (declaration) name.
//
// shortName is the one you'd use in a "package {name}" declaration; pkg is the full import path.
//
// Only the **basename** is mangled — kebab-cased (`-` separated) with the Go ruleset (ASCII folding and initialisms),
// like [GoMangler.File] but with "-" instead of "_".
//
// A leading directory (only "/" is a separator here — a package path is not a filesystem path; trim any trailing "/"
// beforehand) is reconducted verbatim. pkg is `{dir}/{x-y-z}` (or `{x-y-z}`); shortName is the last "-" segment (`z`).
//
//	"github.com/toktok/@alpha-beta" -> shortName "beta", pkg "github.com/toktok/at-alpha-beta"
func (g GoMangler) Package(pth string) (shortName string, pkg string) {
	shortName, pkg, _ = g.packageParts(pth)

	return shortName, pkg
}

// PackageWithParts is like [GoMangler.Package], but also returns the parts of the mangled basename (`[x, y, z]`).
// shortName is the last of these.
//
// Useful when the caller wants to derive a deconflicted import alias from the other significant parts of the package.
func (g GoMangler) PackageWithParts(pth string) (shortName string, pkg string, parts []string) {
	return g.packageParts(pth)
}

// pathBaseKebab splits a "/"-separated path into its verbatim directory prefix and its basename mangled to kebab (fold
// + initialisms).
//
// The dirname is trusted and left untouched (no per-element path checks); an empty kebab (all-separators basename) is
// acceptable.
// Shared by Package and Module.
func (g GoMangler) pathBaseKebab(pth string) (dir, kebab string) {
	pth = strings.TrimRight(pth, "/")

	base := pth
	if i := strings.LastIndexByte(pth, '/'); i >= 0 { // a package/module path uses "/" only, never "\"
		dir, base = pth[:i+1], pth[i+1:]
	}

	return dir, g.identifier(base, TargetKebab())
}

func (g GoMangler) packageParts(pth string) (shortName string, pkg string, parts []string) {
	dir, kebab := g.pathBaseKebab(pth)
	if kebab == "" {
		return "", dir, nil
	}

	// The package short name is the last "-" segment; repair that segment (reflected back into pkg).
	parts = strings.Split(kebab, "-")
	last := len(parts) - 1
	if repaired := repairPackageShort(parts[last]); repaired != parts[last] {
		parts[last] = repaired
		kebab = strings.Join(parts, "-")
	}

	return parts[last], dir + kebab, parts
}

// reservedPackageNames are names the go toolchain treats specially (the main package, and the vendor/internal/testdata
// directories).
//
// A package short name that matches one gets "pkg" appended.
var reservedPackageNames = map[string]struct{}{
	"main":     {},
	"vendor":   {},
	"internal": {},
	"testdata": {},
}

// repairPackageShort fixes a package short name that would confuse the go toolchain:
//   - a reserved name gets "pkg" appended directly (no separator): "main" -> "mainpkg";
//   - a bare major-version element becomes "version<N>": "v2" -> "version2", "V10" -> "version10".
func repairPackageShort(short string) string {
	if _, ok := reservedPackageNames[short]; ok {
		return short + "pkg"
	}
	if digits := majorVersionDigits(short); digits != "" {
		return "version" + digits
	}

	return short
}

// majorVersionDigits returns the digits of a "v<N>"/"V<N>" element (matching ^[vV]\d+$), or "".
func majorVersionDigits(short string) string {
	if len(short) < 2 || (short[0] != 'v' && short[0] != 'V') {
		return ""
	}
	for i := 1; i < len(short); i++ {
		if short[i] < '0' || short[i] > '9' {
			return ""
		}
	}

	return short[1:]
}

// reservedWindowsNames are device names Windows forbids as a file/directory name (case-insensitively), which would
// break a module that maps to a directory on a Windows filesystem.
//
// Checked lowercased (the kebab is already lower-case).
// Additional to the package repairs, for modules only.
var reservedWindowsNames = map[string]struct{}{
	"con": {}, "prn": {}, "aux": {}, "nul": {},
	"com1": {}, "com2": {}, "com3": {}, "com4": {}, "com5": {}, "com6": {}, "com7": {}, "com8": {}, "com9": {},
	"lpt1": {}, "lpt2": {}, "lpt3": {}, "lpt4": {}, "lpt5": {}, "lpt6": {}, "lpt7": {}, "lpt8": {}, "lpt9": {},
}

// repairModuleShort is [repairPackageShort] plus a Windows device-name repair ("con" -> "conpkg").
func repairModuleShort(short string) string {
	if _, ok := reservedWindowsNames[short]; ok {
		return short + "pkg"
	}

	return repairPackageShort(short)
}

// Module produces a legit go module path.
//
// It mangles only the basename (kebab-cased, with the Go ruleset), reconducting the "/"-separated directory prefix
// verbatim — the same shape as [GoMangler.Package]'s pkg — with the same reserved name and major-version repairs,
// plus a repair of Windows device names (con, nul, com1…, lpt1…).
//
// Note: the major-version repair means a trailing "v2" becomes "version2", so an actual semantic-import-versioning
// suffix (".../repo/v2") must be appended by the caller *after* Module, not passed through it.
func (g GoMangler) Module(pth string) string {
	dir, kebab := g.pathBaseKebab(pth)
	if kebab == "" {
		return dir
	}

	// A module path element is repaired as a whole: a directory "my-con" is legal — only bare "con" is a Windows device
	// name, only bare "v2" is a version, etc.
	return dir + repairModuleShort(kebab)
}

// File produces a valid go file name, transforming any trailing segment that bears semantics to the go build system.
//
// Mangling applies only to the file **stem**: the directory prefix (either "/" or "\" separated) and any existing
// extension are reconducted verbatim; the stem is lower-cased and snakized (no ".go" is added).
//
// When the last snake segment is a reserved GOOS/GOARCH/test suffix (which would make the file build-constrained), a
// repair suffix is appended (default "swagger"):
//
//   - "test.go"          -> "test_swagger.go"
//   - "config_linux"     -> "config_linux_swagger"
//   - "some/dir/MyModel" -> "some/dir/my_model"
//   - "IPv4Config.json"  -> "ipv4_config.json"
func (g GoMangler) File(input string) string {
	// isolate the stem; the directory prefix and the extension are reconducted verbatim.
	dir := ""
	base := input
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		dir, base = base[:i+1], base[i+1:]
	}
	ext := ""
	stem := base
	if dot := strings.LastIndexByte(base, '.'); dot > 0 { // dot > 0: keep a leading-dot (hidden) name whole
		ext, stem = base[dot:], base[:dot]
	}

	// identifier merges break-crossing initialisms so "IPv4" snakizes to "ipv4", not "i_pv4".
	return dir + g.repairFileSuffix(g.orFallback(g.identifier(stem, TargetSnake()), TargetSnake())) + ext
}

// repairFileSuffix appends the file repair suffix (default "swagger") when the last snake segment is a reserved
// GOOS/GOARCH/test suffix, so the result is not accidentally build-constrained.
func (g GoMangler) repairFileSuffix(snake string) string {
	if snake == "" {
		return snake
	}

	last := snake[strings.LastIndexByte(snake, '_')+1:] // whole string when there is no "_"
	if _, ok := g.fileSuffixes[last]; ok {
		return snake + "_" + g.fileRepairSuffix
	}

	return snake
}

// ConstName produces a valid exported Go identifier from an arbitrary value (e.g. an enum member).
//
// Every number in the value is verbalized ("0.25" -> "one quarter", "300" -> "three hundred") and the result is turned
// into an exported identifier.
// Type-name prefixing of enum members (Color + Red -> ColorRed) is the code generator's job.
//
//	ConstName("0.25") == "OneQuarter"   ConstName("300") == "ThreeHundred"   ConstName("read only") == "ReadOnly"
func (g GoMangler) ConstName(value string, opts ...ValueOption) string {
	_ = opts // TODO: value options (symbol / keep-digits / rune-name policies)

	return g.IdentExported(g.n.NumberWords(value))
}

// verbalizeLeadingNumber verbalizes a *leading* numeric token so an identifier never starts with a digit: when the
// first token of str is made of digits it is replaced by its words ("12 angry men" -> "twelve angry men"), while
// interior numbers are left as-is ("variable 12" is unchanged, becoming "Variable12").
//
// Used by the Ident* methods (ConstName verbalizes every number).
func (g GoMangler) verbalizeLeadingNumber(str string) string {
	// Find the first token's byte offset (skipping leading, elided separators) without allocating a []rune — range
	// decodes runes in place.
	// The first non-separator rune decides: a number iff a digit.
	start := -1
	for i, r := range str {
		if defaultTokenSeparator(r) {
			continue
		}
		if isASCIIDigit(r) {
			start = i
		}

		break
	}
	if start < 0 {
		return str // first token is not a number — fast path, nothing allocated
	}

	// Extend over the leading numeric run (ASCII digits + one interior decimal point).
	end, seenDot := start, false
loop:
	for end < len(str) {
		switch c := str[end]; {
		case c >= '0' && c <= '9':
			end++
		case c == '.' && !seenDot && end+1 < len(str) && str[end+1] >= '0' && str[end+1] <= '9':
			seenDot, end = true, end+1
		default:
			break loop
		}
	}

	return str[:start] + g.n.NumberWords(str[start:end]) + " " + str[end:]
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

// ToAscii transforms a string to plain ASCII: Latin diacritics are folded (café → cafe), combining
// marks stripped, and any remaining non-ASCII rune is replaced by its phonetic Unicode-name word
// (π → pi, 😀 → grinning face), space-separated so it reads as words. Runes with no known word (CJK
// ideographs, decorative symbols) are dropped.
//
// This works best for European languages; it falls back to [UnicodeName] for other scripts and emoji.
func ToAscii[T ~string | ~[]byte](s T) string {
	in := string(s)
	if isASCII(in) {
		return in
	}

	var b strings.Builder
	b.Grow(len(in))
	for _, r := range in {
		switch {
		case r < utf8.RuneSelf:
			b.WriteRune(r)
		case isCombiningMark(r):
			// strip
		default:
			if f, ok := asciiFold[r]; ok {
				b.WriteString(f)
			} else if v, ok := numbers.RuneNumber(r); ok {
				b.WriteByte(' ')
				b.WriteString(formatNumeral(v)) // numeral rune → plain number ("½" → "0.5"), not wording
				b.WriteByte(' ')
			} else if w, ok := runewords.Word(r); ok {
				b.WriteByte(' ')
				b.WriteString(w)
				b.WriteByte(' ')
			} // else: dropped
		}
	}

	return strings.Join(strings.Fields(b.String()), " ")
}

// formatNumeral renders a Unicode numeral's value as a plain ASCII number, capped at 3 decimals and with
// trailing zeros trimmed: 0.5 → "0.5", 7 → "7", 1/7 (0.142857…) → "0.143". Used by the asciify tier, which
// renders numerals plainly ("½" → "0.5"), unlike the numbers engine which spells them ("½" → "one half").
func formatNumeral(v float64) string {
	const decimals = 3
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if strings.ContainsRune(s, '.') {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}

	return s
}

// Ascii returns the plain-ASCII equivalent of a single rune bearing a diacritic (é → "e", ñ → "n"),
// the rune itself if already ASCII, or "" if it has no ASCII folding (non-Latin letters, symbols,
// emoji — use [UnicodeName] for those, combining marks fold to "").
func Ascii[T ~rune | ~byte](r T) string {
	c := rune(r)
	if c < utf8.RuneSelf {
		return string(c)
	}
	if s, ok := asciiFold[c]; ok {
		return s
	}

	return ""
}

// UnicodeName returns a lowercase phonetic word for a rune with no ASCII folding — its distinctive
// Unicode-name fragment (π → "pi", 😀 → "grinning face", ж → "zhe"). ASCII runes are returned as-is;
// runes the mangler elides (CJK ideographs, combining marks, decorative symbols) return "".
func UnicodeName[T ~rune | ~byte](r T) string {
	c := rune(r)
	if c < utf8.RuneSelf {
		return string(c)
	}
	if w, ok := runewords.Word(c); ok {
		return w
	}

	return ""
}

// isASCII reports whether s contains only ASCII bytes.
func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}
