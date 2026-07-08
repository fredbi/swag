package mangling

import (
	"fmt"
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/go-openapi/swag/mangling/v2/numbers"
	"github.com/go-openapi/testify/v2/assert"
)

func TestMangler(t *testing.T) {
	t.Parallel()

	t.Run("with defaults", func(t *testing.T) {
		t.Parallel()

		m := MakeMangler()

		for tc := range manglerTestCases() {
			t.Run(tc.name, testMangler(m, testModeDefaultMangler, tc))
		}
	})

	t.Run("with ASCII folding", func(t *testing.T) {
		t.Parallel()

		m := MakeMangler(WithASCIIFolding(true))

		for tc := range manglerTestCases() {
			t.Run(tc.name, testMangler(m, testModeASCIIMangler, tc))
		}
	})
}

func TestGoMangler(t *testing.T) {
	t.Parallel()

	// The GoMangler reuses the same harness.
	//
	// It satisfies the [mangler] interface through its embedded Mangler, with ASCII folding on by default.
	// (Initialisms and the Go ident targets come later — the go-specific casings are not exercised yet.)
	m := MakeGoMangler()

	for tc := range manglerTestCases() {
		t.Run(tc.name, testMangler(m, testModeDefaultGoMangler, tc))
	}
}

func TestGoManglerFile(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler()

	for tc := range goFileCases() {
		t.Run(tc.in, func(t *testing.T) {
			assert.EqualTf(t, tc.out, g.File(tc.in), "File(%q)", tc.in)
		})
	}
}

func TestGoManglerPackage(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler()

	for tc := range goPackageCases() {
		t.Run(tc.in, func(t *testing.T) {
			short, pkg, parts := g.PackageWithParts(tc.in)
			assert.EqualTf(t, tc.short, short, "short for %q", tc.in)
			assert.EqualTf(t, tc.pkg, pkg, "pkg for %q", tc.in)
			assert.Truef(t, slices.Equal(tc.parts, parts), "parts for %q: got %v, want %v", tc.in, parts, tc.parts)

			short2, pkg2 := g.Package(tc.in) // Package returns the same short/pkg
			assert.EqualTf(t, short, short2, "Package short for %q", tc.in)
			assert.EqualTf(t, pkg, pkg2, "Package pkg for %q", tc.in)
		})
	}
}

func TestGoManglerModule(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler()

	for tc := range goModuleCases() {
		t.Run(tc.in, func(t *testing.T) {
			assert.EqualTf(t, tc.out, g.Module(tc.in), "Module(%q)", tc.in)
		})
	}
}

func TestGoManglerConstName(t *testing.T) {
	t.Parallel()

	t.Run("general constants", func(t *testing.T) {
		t.Parallel()

		g := MakeGoMangler()

		for tc := range goConstNameCases() {
			t.Run(tc.in, func(t *testing.T) {
				assert.EqualTf(t, tc.out, g.ConstName(tc.in), "ConstName(%q)", tc.in)
			})
		}
	})

	t.Run("with NumberOptions", func(t *testing.T) {
		t.Parallel()

		// number options flow into ConstName via WithGoNumberOptions
		g := MakeGoMangler(WithGoNumberOptions(
			numbers.WithSpecialNumbers(map[string]string{"3.1415": "pi", "2.718": "e"}),
		))

		assert.EqualT(t, "Pi", g.ConstName("3.1415"))
		assert.EqualT(t, "E", g.ConstName("2.718"))
		assert.EqualT(t, "OneQuarter", g.ConstName("0.25"))                             // non-special still works
		assert.EqualT(t, "ThreeDotOneFourOneFive", MakeGoMangler().ConstName("3.1415")) // default: no specials
	})

	t.Run("with rune names", func(t *testing.T) {
		t.Parallel()

		g := MakeGoMangler()

		for tc := range goRuneNameCases() {
			t.Run(tc.in, func(t *testing.T) {
				assert.EqualTf(t, tc.out, g.ConstName(tc.in), "ConstName(%q)", tc.in)
			})
		}
	})

	t.Run("with ascii folding off", func(t *testing.T) {
		t.Parallel()

		// asciify off preserves the original runes (no folding, no naming).
		raw := MakeGoMangler(WithManglerOptions(WithASCIIFolding(false)))
		assert.EqualT(t, "Café", raw.IdentExported("café"))
	})
}

func TestASCIIUtilities(t *testing.T) {
	t.Parallel()

	// ToASCII: fold diacritics, name the rest, drop the unnameable.
	t.Run("ToASCII", func(t *testing.T) {
		t.Parallel()

		for tc := range toASCIICases() {
			assert.EqualTf(t, tc.out, ToASCII(tc.in), "ToASCII(%q)", tc.in)
		}
	})

	// RuneToASCII: single-rune diacritic / digit fold only.
	t.Run("RuneToASCII", func(t *testing.T) {
		t.Parallel()

		for tc := range runeToASCIICases() {
			assert.EqualTf(t, tc.want, RuneToASCII(tc.in), "RuneToASCII(%q)", tc.in)
		}
	})

	// RuneShortName: phonetic word for non-foldable runes.
	t.Run("RuneShortName", func(t *testing.T) {
		t.Parallel()

		for tc := range runeShortNameCases() {
			assert.EqualTf(t, tc.want, RuneShortName(tc.in), "RuneShortName(%q)", tc.in)
		}
	})
}

// =============================================
// general purpose test harness
// =============================================

func testMangler(m mangler, mode testMode, tc manglerTestCase) func(*testing.T) {
	return func(t *testing.T) {
		casings := []testedCasing{
			testedPascal,
			testedCamel,
			testedSnake,
			testedKebab,
			testedHuman,
			testedTitle,
			testedAllCaps,
		}
		if mode == testModeDefaultGoMangler {
			casings = append(casings, testedGoExported, testedGoUnexported)
		}

		expectator := tc.expected(mode)
		for _, casing := range casings {
			expected := expectator[casing]
			if expected == "" {
				continue // no expectation for this casing in this mode
			}

			asserted := casing.MethodFor(m)
			assert.EqualTf(t, expected, asserted(tc.input), "%v-case test", casing)
		}
	}
}

type mangler interface {
	Pascalize(str string) string
	Camelize(str string) string
	Snakize(str string) string
	Kebabize(str string) string
	Humanize(str string) string
	Titleize(str string) string
	AllCaps(str string) string
}

type testedCasing string

func (tc testedCasing) MethodFor(m mangler) func(string) string {
	switch tc {
	case testedPascal:
		return m.Pascalize
	case testedCamel:
		return m.Camelize
	case testedSnake:
		return m.Snakize
	case testedKebab:
		return m.Kebabize
	case testedHuman:
		return m.Humanize
	case testedTitle:
		return m.Titleize
	case testedAllCaps:
		return m.AllCaps
	default:
		gomangler, ok := m.(GoMangler)
		if !ok {
			gomanglerPtr, ok := m.(*GoMangler)
			if !ok {
				panic(fmt.Sprintf("dev error: invalid testedCasing value: %v", tc))
			}
			gomangler = *gomanglerPtr
		}

		switch tc {
		case testedGoUnexported:
			return gomangler.IdentUnexported
		case testedGoExported:
			return gomangler.IdentExported
		default:
			panic(fmt.Sprintf("dev error: invalid testedCasing value: %v", tc))
		}
	}
}

const (
	testedPascal  testedCasing = "pascal"
	testedCamel   testedCasing = "camel"
	testedSnake   testedCasing = "snake"
	testedKebab   testedCasing = "kebab"
	testedHuman   testedCasing = "human"
	testedTitle   testedCasing = "title"
	testedAllCaps testedCasing = "all-caps"

	// go-specifics
	testedGoUnexported testedCasing = "go-unexported"
	testedGoExported   testedCasing = "go-exported"
)

type testMode int8

const (
	testModeDefaultMangler testMode = iota
	testModeASCIIMangler
	testModeDefaultGoMangler // defaults with ASCII, initialisms
)

type manglerTestCase struct {
	name     string
	input    string
	expected func(testMode) map[testedCasing]string
}

// =============================================
// recasing and identifiers
// =============================================

//nolint:maintidx // a flat table of test-case data, not algorithmic complexity
func manglerTestCases() iter.Seq[manglerTestCase] {
	return slices.Values([]manglerTestCase{
		{
			name:  "simple sentence",
			input: "sample text",
			expected: func(_ testMode) map[testedCasing]string {
				return map[testedCasing]string{
					testedPascal:  "SampleText",
					testedCamel:   "sampleText",
					testedSnake:   "sample_text",
					testedKebab:   "sample-text",
					testedHuman:   "Sample text",
					testedTitle:   "Sample Text",
					testedAllCaps: "SAMPLE_TEXT",
				}
			},
		},
		{
			name:  "with elided separators",
			input: "simple,punctuated ;list _ of -words",
			expected: func(_ testMode) map[testedCasing]string {
				return map[testedCasing]string{
					testedPascal:  "SimplePunctuatedListOfWords",
					testedCamel:   "simplePunctuatedListOfWords",
					testedSnake:   "simple_punctuated_list_of_words",
					testedKebab:   "simple-punctuated-list-of-words",
					testedHuman:   "Simple punctuated list of words",
					testedTitle:   "Simple Punctuated List Of Words",
					testedAllCaps: "SIMPLE_PUNCTUATED_LIST_OF_WORDS",
				}
			},
		},
		{
			name:  "with symbols",
			input: "simple#symbolic @list . of$ words",
			expected: func(_ testMode) map[testedCasing]string {
				return map[testedCasing]string{
					testedPascal:  "SimpleHashSymbolicAtListDotOfDollarWords",
					testedCamel:   "simpleHashSymbolicAtListDotOfDollarWords",
					testedSnake:   "simple_hash_symbolic_at_list_dot_of_dollar_words",
					testedKebab:   "simple-hash-symbolic-at-list-dot-of-dollar-words",
					testedHuman:   "Simple hash symbolic at list dot of dollar words",
					testedTitle:   "Simple Hash Symbolic At List Dot Of Dollar Words",
					testedAllCaps: "SIMPLE_HASH_SYMBOLIC_AT_LIST_DOT_OF_DOLLAR_WORDS",
				}
			},
		},
		{
			name:  "with non-leading digits",
			input: "simple 1 text 2",
			expected: func(_ testMode) map[testedCasing]string {
				return map[testedCasing]string{
					testedPascal:  "Simple1Text2",
					testedCamel:   "simple1Text2",
					testedSnake:   "simple1_text2",
					testedKebab:   "simple1-text2",
					testedHuman:   "Simple1 text2",
					testedTitle:   "Simple1 Text2",
					testedAllCaps: "SIMPLE1_TEXT2",
				}
			},
		},
		{
			name:  "with leading digits",
			input: "0simple 1 text 2",
			expected: func(_ testMode) map[testedCasing]string {
				return map[testedCasing]string{
					testedPascal:  "0Simple1Text2",
					testedCamel:   "0simple1Text2",
					testedSnake:   "0_simple1_text2",
					testedKebab:   "0-simple1-text2",
					testedHuman:   "0 Simple1 text2",
					testedTitle:   "0 Simple1 Text2",
					testedAllCaps: "0_SIMPLE1_TEXT2",
				}
			},
		},

		{
			name:  "with diacritics",
			input: "café crème",
			expected: func(mode testMode) map[testedCasing]string {
				if mode == testModeDefaultMangler {
					// base Mangler: folding off, diacritics preserved
					return map[testedCasing]string{
						testedPascal:  "CaféCrème",
						testedCamel:   "caféCrème",
						testedSnake:   "café_crème",
						testedKebab:   "café-crème",
						testedHuman:   "Café crème",
						testedTitle:   "Café Crème",
						testedAllCaps: "CAFÉ_CRÈME",
					}
				}

				// ASCII folding on (ASCII mode and GoMangler default)
				return map[testedCasing]string{
					testedPascal:  "CafeCreme",
					testedCamel:   "cafeCreme",
					testedSnake:   "cafe_creme",
					testedKebab:   "cafe-creme",
					testedHuman:   "Cafe creme",
					testedTitle:   "Cafe Creme",
					testedAllCaps: "CAFE_CREME",
				}
			},
		},
		{
			name:  "with combining diacritics",
			input: "cafe\u0301 cre\u0300me", // NFD (decomposed) form of cafe/creme
			expected: func(_ testMode) map[testedCasing]string {
				// Combining marks are never valid identifier runes, so they are stripped in EVERY mode (not only when ASCII folding
				// is on) \u2014 the decomposed diacritics vanish regardless.
				return map[testedCasing]string{
					testedPascal:  "CafeCreme",
					testedCamel:   "cafeCreme",
					testedSnake:   "cafe_creme",
					testedKebab:   "cafe-creme",
					testedHuman:   "Cafe creme",
					testedTitle:   "Cafe Creme",
					testedAllCaps: "CAFE_CREME",
				}
			},
		},
		{
			name:  "with CJK",
			input: "日本語 text",
			expected: func(mode testMode) map[testedCasing]string {
				if mode == testModeDefaultMangler {
					// base Mangler: folding off, non-Latin scripts preserved
					return map[testedCasing]string{
						testedPascal:  "日本語Text",
						testedCamel:   "日本語Text",
						testedSnake:   "日本語_text",
						testedKebab:   "日本語-text",
						testedHuman:   "日本語 text",
						testedTitle:   "日本語 Text",
						testedAllCaps: "日本語_TEXT",
					}
				}

				// folding on: CJK ideographs have no phonetic name -> elided by the rune-name stage
				return map[testedCasing]string{
					testedPascal:  "Text",
					testedCamel:   "text",
					testedSnake:   "text",
					testedKebab:   "text",
					testedHuman:   "Text",
					testedTitle:   "Text",
					testedAllCaps: "TEXT",
				}
			},
		},

		// Regression cases locking the numeral/digit/mark contract fixes surfaced by FuzzGoIdent.
		{
			name:  "with vulgar fraction (No numeral rune)",
			input: "½ over 200",
			expected: func(mode testMode) map[testedCasing]string {
				if mode == testModeDefaultMangler {
					// base Mangler, folding off: the numeral rune is not a valid ident char and is dropped
					return map[testedCasing]string{testedPascal: "Over200", testedCamel: "over200", testedSnake: "over200"}
				}

				// folding on: a numeral rune is spelled out as words (a name reads better than "0Dot5")
				m := map[testedCasing]string{testedPascal: "OneHalfOver200", testedCamel: "oneHalfOver200", testedSnake: "one_half_over200"}
				if mode == testModeDefaultGoMangler {
					m[testedGoExported], m[testedGoUnexported] = "OneHalfOver200", "oneHalfOver200"
				}

				return m
			},
		},
		{
			name:  "with roman numeral (Nl numeral rune)",
			input: "Ⅶ legions",
			expected: func(mode testMode) map[testedCasing]string {
				if mode == testModeDefaultMangler {
					return map[testedCasing]string{testedPascal: "Legions", testedCamel: "legions", testedSnake: "legions"}
				}

				m := map[testedCasing]string{testedPascal: "SevenLegions", testedCamel: "sevenLegions", testedSnake: "seven_legions"}
				if mode == testModeDefaultGoMangler {
					m[testedGoExported], m[testedGoUnexported] = "SevenLegions", "sevenLegions"
				}

				return m
			},
		},
		{
			name:  "with unicode decimal digit (Nd, digit-offset to ASCII)",
			input: "item ٧ code", // Arabic-Indic digit seven
			expected: func(mode testMode) map[testedCasing]string {
				if mode == testModeDefaultMangler {
					// base Mangler, folding off: a non-ASCII digit is preserved (valid non-leading ident rune)
					return map[testedCasing]string{testedPascal: "Item٧Code", testedCamel: "item٧Code", testedSnake: "item٧_code"}
				}

				// folding on: digit-offset converts it to the ASCII digit, so it behaves exactly like '7'
				m := map[testedCasing]string{testedPascal: "Item7Code", testedCamel: "item7Code", testedSnake: "item7_code"}
				if mode == testModeDefaultGoMangler {
					m[testedGoExported], m[testedGoUnexported] = "Item7Code", "item7Code"
				}

				return m
			},
		},
		{
			// a leading non-ASCII digit can't start an identifier, so the GoMangler verbalizes it (both modes)
			name:     "with leading unicode decimal digit (Nd)",
			input:    "٧ lives",
			expected: goIdents("SevenLives", "sevenLives"),
		},
		{
			// an orphan combining mark (dropped) then a symbol: the symbol word must be lower-cased for the unexported ident —
			// regression, it used to come out "Bang" because the empty mark token consumed the first-word casing slot.
			name:  "with orphan combining mark and symbol",
			input: "֮!", // Hebrew accent (Mn) + '!'
			expected: func(mode testMode) map[testedCasing]string {
				m := map[testedCasing]string{testedPascal: "Bang", testedCamel: "bang", testedSnake: "bang"}
				if mode == testModeDefaultGoMangler {
					m[testedGoExported], m[testedGoUnexported] = "Bang", "bang"
				}

				return m
			},
		},

		// GoMangler initialisms (only the go-ident casings assert; neutral casings do not recognize initialisms and are left
		// unset here).
		{
			name:     "simple initialisms",
			input:    "get http response id",
			expected: goIdents("GetHTTPResponseID", "getHTTPResponseID"),
		},
		{
			name:     "leading initialism (unexported lowercases)",
			input:    "http get",
			expected: goIdents("HTTPGet", "httpGet"),
		},
		{
			name:     "token-breaking initialism (IPv4)",
			input:    "ipv4 address",
			expected: goIdents("IPv4Address", "ipv4Address"),
		},
		{
			name:     "pluralized initialism (IDs)",
			input:    "userIDs",
			expected: goIdents("UserIDs", "userIDs"),
		},
		{
			name:     "pluralized ambiguous (IDS is not plural)",
			input:    "IDS",
			expected: goIdents("Ids", "ids"),
		},
		{
			name:     "invariant initialism (DNS)",
			input:    "dns lookup",
			expected: goIdents("DNSLookup", "dnsLookup"),
		},
		{
			name:     "reserved keyword repair (unexported only)",
			input:    "type",
			expected: goIdents("Type", "typeVar"),
		},
		{
			name:     "reserved builtin repair (unexported only)",
			input:    "append",
			expected: goIdents("Append", "appendVar"),
		},
		{
			name:     "leading digit verbalized",
			input:    "12 angry men",
			expected: goIdents("TwelveAngryMen", "twelveAngryMen"),
		},
		{
			name:     "interior digit kept",
			input:    "variable 12",
			expected: goIdents("Variable12", "variable12"),
		},
		{
			name:     "leading fraction verbalized",
			input:    "0.1 index",
			expected: goIdents("OneTenthIndex", "oneTenthIndex"),
		},
		{
			name:     "interior decimal keeps digits, dot verbalized",
			input:    "index 0.1",
			expected: goIdents("Index0Dot1", "index0Dot1"),
		},
		{
			name:     "interior symbol verbalized",
			input:    "how many? 12",
			expected: goIdents("HowManyQuestion12", "howManyQuestion12"),
		},
		{
			name:     "leading combining mark",
			input:    "́abc",
			expected: goIdents("Abc", "abc"),
		},
	})
}

// goIdents builds an expectation that asserts only the go-ident casings, and only in GoMangler mode.
func goIdents(exported, unexported string) func(testMode) map[testedCasing]string {
	return func(mode testMode) map[testedCasing]string {
		if mode != testModeDefaultGoMangler {
			return nil
		}

		return map[testedCasing]string{
			testedGoExported:   exported,
			testedGoUnexported: unexported,
		}
	}
}

// =============================================
// File
// =============================================

// inOutCase is a simple input → expected-output test case, shared by the string-in/string-out Go targets.
type inOutCase struct{ in, out string }

func goFileCases() iter.Seq[inOutCase] {
	return slices.Values([]inOutCase{
		{"MyModel", "my_model"},
		{"my model", "my_model"},
		{"test.go", "test_swagger.go"},           // reserved: test
		{"config_linux", "config_linux_swagger"}, // reserved: GOOS
		{"arm64.tmpl", "arm64_swagger.tmpl"},     // reserved: GOARCH, extension preserved
		{"windows", "windows_swagger"},           // whole stem is a GOOS
		{"handler_test", "handler_test_swagger"},
		{"user_id", "user_id"},                            // "id" is an initialism but snake lowercases it; not a file suffix
		{"some/dir/MyModel", "some/dir/my_model"},         // directory prefix reconducted verbatim
		{`win\dir\MyModel.json`, `win\dir\my_model.json`}, // backslash dir + extension reconducted verbatim
		{"IPv4Config.json", "ipv4_config.json"},           // break-crossing initialism merged
		{"HTTPServer", "http_server"},                     // initialism lowercased in snake
		{"café résumé", "cafe_resume"},                    // ASCII folded
	})
}

// =============================================
// Package
// =============================================

type goPackageCase struct {
	in, short, pkg string
	parts          []string
}

func goPackageCases() iter.Seq[goPackageCase] {
	return slices.Values([]goPackageCase{
		{"MyPackage", "package", "my-package", []string{"my", "package"}},
		{"github.com/go-redis/redis", "redis", "github.com/go-redis/redis", []string{"redis"}},
		{"github.com/toktok/@alpha-beta", "beta", "github.com/toktok/at-alpha-beta", []string{"at", "alpha", "beta"}},
		{"github.com/user/GoThing/", "thing", "github.com/user/go-thing", []string{"go", "thing"}}, // trailing "/" trimmed
		{"SomeHTTPClient", "client", "some-http-client", []string{"some", "http", "client"}},       // initialism lowercased
		{"path/to/IPv4Utils", "utils", "path/to/ipv4-utils", []string{"ipv4", "utils"}},            // break-crossing initialism merged
		{"café", "cafe", "cafe", []string{"cafe"}},                                                 // ASCII folded

		// go-toolchain short-name repairs
		{"main", "mainpkg", "mainpkg", []string{"mainpkg"}},                                                 // reserved package name
		{"github.com/user/internal", "internalpkg", "github.com/user/internalpkg", []string{"internalpkg"}}, // reserved dir
		{"pkg/vendor", "vendorpkg", "pkg/vendorpkg", []string{"vendorpkg"}},
		{"testdata", "testdatapkg", "testdatapkg", []string{"testdatapkg"}},
		{"xxxx/v2", "version2", "xxxx/version2", []string{"version2"}}, // major-version element
		{"foo/V10", "version10", "foo/version10", []string{"version10"}},
	})
}

// =============================================
// Module
// =============================================

func goModuleCases() iter.Seq[inOutCase] {
	return slices.Values([]inOutCase{
		{"MyModule", "my-module"},
		{"github.com/user/MyRepo", "github.com/user/my-repo"},        // dir kept verbatim
		{"github.com/user/repo/v2", "github.com/user/repo/version2"}, // load-bearing version neuterized (caller re-adds /vN)
		{"example.com/main", "example.com/mainpkg"},                  // a "main" module isn't go-gettable
		{"example.com/internal", "example.com/internalpkg"},          // reserved dir
		{"example.com/testdata", "example.com/testdatapkg"},          // reserved dir
		{"example.com/con", "example.com/conpkg"},                    // Windows device name
		{"host.tld/COM1", "host.tld/com1pkg"},                        // case-insensitive
		{"example.com/my-con", "example.com/my-con"},                 // whole element is legal → not touched
		{"example.com/my-v2", "example.com/my-v2"},                   // not a bare version element
		{"café", "cafe"}, // ASCII folded
	})
}

// =============================================
// ConstName
// =============================================

func goConstNameCases() iter.Seq[inOutCase] {
	return slices.Values([]inOutCase{
		{"read only", "ReadOnly"},
		{"1", "One"},
		{"300", "ThreeHundred"},
		{"0.25", "OneQuarter"},             // fraction
		{"0.1", "OneTenth"},                // fraction
		{"-5", "MinusFive"},                // sign
		{"3.14", "ThreeDotOneFour"},        // non-fraction decimal
		{"status 200", "StatusTwoHundred"}, // every number verbalized
		// regression: numeral runes and non-ASCII digits verbalize into valid const names (FuzzGoIdent)
		{"½ off", "OneHalfOff"}, // No numeral rune
		{"٧", "Seven"},          // Nd non-ASCII digit (Arabic-Indic), via digit-offset
		{"Ⅶ", "Seven"},          // Nl roman numeral
		{"①", "One"},            // No circled digit
		{"50%", "FiftyPercent"},
		// regression test: an integer too large for int64 is spelled digit by digit, so the const name stays a valid
		// (non-digit-leading) identifier rather than raw digits.
		{"9999999999999999999", strings.Repeat("Nine", 19)},
	})
}

func goRuneNameCases() iter.Seq[inOutCase] {
	return slices.Values([]inOutCase{
		{"café", "Cafe"},                       // diacritic fold
		{"naïve", "Naive"},                     // diaeresis fold
		{"π", "Pi"},                            // non-Latin letter -> phonetic name
		{"σ field", "SigmaField"},              // named rune re-segments and re-cases as a word
		{"δ plus ε", "DeltaPlusEpsilon"},       // multiple named runes
		{"grinning 😀", "GrinningGrinningFace"}, // single-codepoint emoji named
		{"Ω max", "OmegaMax"},                  // uppercase Greek
		{"日本 value", "Value"},                  // CJK ideographs elided -> clean ASCII
	})
}

// =============================================
// ASCII utilities
// =============================================

// runeStringCase is a single-rune input → expected-output test case.
type runeStringCase struct {
	in   rune
	want string
}

func toASCIICases() iter.Seq[inOutCase] {
	return slices.Values([]inOutCase{
		{"café", "cafe"},
		{"naïve", "naive"},
		{"π", "pi"},
		{"😀", "grinning face"},
		{"日", ""}, // CJK dropped
		{"plain ascii", "plain ascii"},
		// non-ASCII decimal digits (Nd) fold to their ASCII value, like the pipeline (item٧ → item7)
		{"٧", "7"}, // Arabic-Indic
		{"๗", "7"}, // Thai
		{"７", "7"}, // fullwidth
		{"item٧", "item7"},
		{"Ⅶ", "7"}, // Nl numeral renders as a plain number
		// diacritic fold spans every Latin block (generated asciiFold): Vietnamese, pinyin, ligatures
		{"Tiếng Việt", "Tieng Viet"},
		{"Nǐ hǎo", "Ni hao"}, // pinyin ǐ→i, ǎ→a
		{"oﬃce", "office"},   // ﬃ ligature → ffi
	})
}

func runeToASCIICases() iter.Seq[runeStringCase] {
	return slices.Values([]runeStringCase{
		{'é', "e"},
		{'ñ', "n"},
		{'A', "A"},
		{'٧', "7"},  // non-ASCII decimal digit (Nd) folds to its ASCII value
		{'๙', "9"},  // Thai
		{'ế', "e"},  // Vietnamese e-circumflex-acute
		{'ơ', "o"},  // horn
		{'œ', "oe"}, // OE ligature
		{'π', ""},   // no diacritic folding -> empty (use RuneShortName)
	})
}

func runeShortNameCases() iter.Seq[runeStringCase] {
	return slices.Values([]runeStringCase{
		{'π', "pi"},
		{'ж', "zhe"},
		{'λ', "lambda"}, // wordOverrides: Unicode's "lamda" -> "lambda"
		{'😀', "grinning face"},
		{'A', "A"}, // ASCII as-is
		{'中', ""},  // elided
	})
}
