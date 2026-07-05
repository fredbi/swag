package mangling

import (
	"fmt"
	"iter"
	"slices"
	"testing"

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
}

func testMangler(m mangler, mode testMode, tc manglerTestCase) func(*testing.T) {
	return func(t *testing.T) {
		for _, casing := range []testedCasing{
			testedPascal,
			testedCamel,
			testedSnake,
			testedKebab,
			testedHuman,
			testedTitle,
			testedAllCaps,
		} {
			var expected string
			expectator := tc.expected(mode)
			expected = expectator[casing]

			if expected == "" {
				t.Skipf("skipped %v", casing)

				continue
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

		// TODO: token split w/ unicode
		// unicode latin (w/ combining diacritics)

		// TODO: mangler with ASCII mode
		// unicode digit (e.g. indian-arabic)
		// unicode letter-number (e.g. roman number)
		// unicode latin (w/ diacritics)
		// unicode latin (w/ upper-case with diacritics)
		// unicode latin (ASCII-fy w/ combining diacritics)

		// TODO: unicode verbalisation
		// unicode arabic (w/ extended arabic signs - expected to be elided)
		// unicode chinese
		// unicode japanese
		// unicode japanese CJK
		// unicode devanagari (no upper case concept)
		// unicode edge-cases: non printable rune, invalid rune, ASCII control char, ...

		// TODO: gomangler mode (default: ASCII on)
		// simple initialism (ID, HTTP)
		// token-breaking initialism (IPv4, IPv6)
		// pluralized initialism (IDs)
		// pluralized initialism (ambiguous: TTLs)
		// leading separators => elided
		// leading unicode non-letters => verbalized

		// TODO: not supported yet
		// leading digits => TODO: require NumberMangler
		// special casing rules (e.g. greek lower-case sigma)
		// unicode emojis
		// unicode graphemes (e.g. flags)
	})
}
