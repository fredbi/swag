package mangling

import (
	"maps"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/swag/mangling/v2/numbers"
)

// GoMangler turns strings into identifiers that abide by go naming conventions.
//
// It embeds a [Mangler] — so every base formatter ([Mangler.Camelize], [Mangler.Snakize], ...) is available — with
// ASCII folding enabled by default, and adds go-aware producers:
//
//   - [GoMangler.IdentExported] / [GoMangler.IdentUnexported]: valid exported / unexported identifiers;
//   - [GoMangler.ConstName]: an exported identifier with numbers verbalized ("0.25" → "OneQuarter");
//   - [GoMangler.File]: a snake_case file name, repaired to avoid a build-constrained suffix;
//   - [GoMangler.Package] / [GoMangler.PackageWithParts] / [GoMangler.Module]: go package and module path elements.
//
// The producers honor a configurable list of initialisms (API, HTTP, ...), resolve collisions with go keywords and
// builtins, and guarantee a valid, non-empty identifier for any input.
type GoMangler struct {
	Mangler
	goOptions

	n        numbers.NumberMangler
	trie     *initialismTrie     // precomputed initialism index (shared, read-only)
	reserved map[string]struct{} // keywords ∪ builtins, for unexported-ident repair
}

// MakeGoMangler returns a value [GoMangler].
func MakeGoMangler(opts ...GoOption) GoMangler {
	var g GoMangler
	g.goOptions = buildGoOptions(g.goOptions, opts)
	g.Mangler.options = g.goOptions.options
	g.tokenizer.tokenOptions = g.goOptions.tokenOptions
	g.n = numbers.MakeNumberMangler(g.numberOpts...)
	g.trie = buildInitialismTrie(g.initialisms)

	g.reserved = make(map[string]struct{}, len(g.keywords)+len(g.builtins))
	maps.Copy(g.reserved, g.keywords)
	maps.Copy(g.reserved, g.builtins)

	return g
}

// NewGoMangler returns a pointer to a [GoMangler].
func NewGoMangler(opts ...GoOption) *GoMangler {
	g := MakeGoMangler(opts...)

	return &g
}

// defaultIdentFallback is the built-in word used when an identifier reduces to nothing and the configured
// [WithGoIdentFallback] word (if any) also reduces to nothing.
//
// It must be a clean word that always survives mangling.
const defaultIdentFallback = "empty"

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
	return g.repairReserved(g.orFallback(g.goIdent(str, TargetCamel()), TargetCamel()))
}

// IdentExported produces a valid exported go variable identifier from a string, possibly containing multiple words.
//
// IdentExported works like [Mangler.Pascalize] with a few go-specific additions:
//   - compiler constraint: identifiers that don't start with a letter get their prefix verbalized (e.g. digits get
//     their numeral wording, symbols get replaced by a word, etc).
//
// Unlike their unexported counterpart, exported identifiers can't conflict with go reserved keywords or builtins.
func (g GoMangler) IdentExported(str string) string {
	return g.orFallback(g.goIdent(str, TargetPascal()), TargetPascal())
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

// ConstName produces a valid exported Go identifier from an arbitrary value (e.g. an enum member).
//
// Every number in the value is verbalized ("0.25" -> "one quarter", "300" -> "three hundred") and the result is turned
// into an exported identifier.
// Type-name prefixing of enum members (Color + Red -> ColorRed) is the code generator's job.
//
//	ConstName("0.25") == "OneQuarter"   ConstName("300") == "ThreeHundred"   ConstName("read only") == "ReadOnly"
func (g GoMangler) ConstName(value string) string {
	// Value-policy options (symbol / keep-digits / rune-name) are deferred; a variadic can be added back later without
	// breaking callers.
	return g.IdentExported(g.n.NumberWords(value))
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

// orFallback guarantees a non-empty identifier: it returns id when non-empty, otherwise the configured fallback word
// mangled at the same target (so it is valid and cased to match — "Empty"/"empty"/snake), and finally the built-in
// [defaultIdentFallback] if even the configured word reduces to nothing.
//
// It is applied by the Go identifier producers (idents, const, file), never inside [GoMangler.identifier] itself —
// Package/Module intentionally allow an empty (dir-only) result.
func (g GoMangler) orFallback(id string, target TargetTransform) string {
	if id != "" {
		return id
	}
	if fb := g.identifier(g.identFallback, target); fb != "" {
		return fb
	}

	return g.identifier(defaultIdentFallback, target)
}

// identifier runs the Go ident pipeline: rune-name expansion → segment → ASCII fold → initialism overlay →
// assemble.
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

// repairModuleShort is [repairPackageShort] plus a Windows device-name repair ("con" -> "conpkg").
func repairModuleShort(short string) string {
	if _, ok := reservedWindowsNames[short]; ok {
		return short + "pkg"
	}

	return repairPackageShort(short)
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

// goIdent runs the full Go-identifier pipeline at the given target casing:
//   - asciify first, so a numeral rune (½, Ⅶ, ②) becomes a number that verbalizeLeadingNumber can catch
//     at the front (identifier re-asciifies idempotently on the resulting ASCII);
//   - verbalize a leading number so the identifier never starts with a digit;
//   - a safety net: elision (invalid UTF-8, combining marks) can strip the rune that shielded a digit,
//     leaving the mangled identifier starting with a digit — verbalize that exposed leading digit too.
func (g GoMangler) goIdent(str string, target TargetTransform) string {
	str = g.asciifyInput(str)
	id := g.identifier(g.verbalizeLeadingNumber(str), target)

	if id != "" {
		if r, _ := utf8.DecodeRuneInString(id); isASCIIDigit(r) || isNonASCIIDigit(r) {
			id = g.identifier(g.verbalizeLeadingNumber(id), target)
		}
	}

	return id
}

// verbalizeLeadingNumber verbalizes a *leading* numeric token so an identifier never starts with a digit: when the
// first token of str is a number it is replaced by its words ("12 angry men" -> "twelve angry men"), while interior
// numbers are left as-is ("variable 12" is unchanged, becoming "Variable12").
//
// It recognizes every kind of leading number: ASCII digit runs, non-ASCII decimal digits (Nd — Arabic-Indic ٧,
// Devanagari, …), and No/Nl numeral runes (½, Ⅶ).
// Non-ASCII digits are converted to ASCII before verbalizing.
//
// Used by the Ident* methods (ConstName verbalizes every number).
func (g GoMangler) verbalizeLeadingNumber(str string) string {
	// The first non-separator rune decides whether the first token is a number.
	start := -1
	var r0 rune
	for i, r := range str {
		if defaultTokenSeparator(r) {
			continue
		}
		start, r0 = i, r

		break
	}
	if start < 0 {
		return str // no content — fast path, nothing allocated
	}

	switch {
	case isASCIIDigit(r0):
		// Leading ASCII digit run with one interior decimal point — byte scan, no allocation.
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

	case isNonASCIIDigit(r0):
		// Leading non-ASCII decimal digits (folding off): convert the run to ASCII, then verbalize.
		var digits []byte
		end, seenDot := start, false
		for end < len(str) {
			r, size := utf8.DecodeRuneInString(str[end:])
			if d, ok := asciiDigit(r); ok {
				digits = append(digits, d)
				end += size

				continue
			}
			if r == '.' && !seenDot {
				digits = append(digits, '.')
				seenDot, end = true, end+size

				continue
			}

			break
		}

		return str[:start] + g.n.NumberWords(string(digits)) + " " + str[end:]

	default:
		if _, ok := numbers.RuneNumber(r0); ok {
			// A leading numeral *rune* (½, Ⅶ, ①) with folding off: verbalize it in place so the ident starts with a letter
			// (folding on already spelled it out upstream, in expandRuneNames).
			w := start + utf8.RuneLen(r0)

			return str[:start] + g.n.NumberWords(str[start:w]) + " " + str[w:]
		}

		return str // first token is not a number
	}
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

// isNonASCIIDigit reports whether r is a decimal digit (Nd) outside the ASCII range.
func isNonASCIIDigit(r rune) bool {
	if r < utf8.RuneSelf {
		return false
	}
	_, ok := asciiDigit(r)

	return ok
}

// formatNumeral renders a Unicode numeral's value as a plain ASCII number.
//
// The returned number string is capped at 3 decimals and with trailing zeros trimmed:
//
//   - 0.5 → "0.5",
//   - 7 → "7",
//   - 1/7 (0.142857…) → "0.143"
//
// Used by the asciify tier, which renders numerals plainly ("½" → "0.5"), unlike the numbers engine which spells
// them ("½" → "one half").
func formatNumeral(v float64) string {
	const decimals = 3
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if strings.ContainsRune(s, '.') {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}

	return s
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

// Precomputed default detection sets for the Go ruleset (shared, read-only after init).
var (
	goKeywordsSet     = toSet(goReservedWords)
	goBuiltinsSet     = toSet(goBuiltins)
	goFileSuffixesSet = toSet(goFileSuffixes)
)

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
