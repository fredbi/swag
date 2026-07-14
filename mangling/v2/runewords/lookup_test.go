// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runewords

import (
	"testing"
	"unicode/utf8"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// codePoint converts a table codepoint (stored as uint32) to a rune, asserting it is in range.
//
// The generator only emits valid codepoints, so this never truncates — the check also catches a corrupt table and
// keeps the uint32→rune conversion honest (gosec G115).
func codePoint(t *testing.T, cp uint32) rune {
	t.Helper()
	require.LessOrEqualf(t, cp, uint32(utf8.MaxRune), "table codepoint U+%X out of range", cp)

	return rune(cp) //nolint:gosec // cp is bounds-checked to utf8.MaxRune just above; the table holds only valid codepoints
}

func TestWordSpotChecks(t *testing.T) {
	t.Parallel()

	covered := map[rune]string{
		'α': "alpha",
		'Ω': "omega",
		'ж': "zhe",
		'€': "euro",
		'❤': "heart",
		'😀': "grinning face",
		'👍': "thumbs up",
		'۩': "sajdah",
	}
	for r, want := range covered {
		got, ok := Word(r)
		assert.Truef(t, ok, "Word(%q) should be covered", r)
		assert.Equalf(t, want, got, "Word(%q)", r)
	}

	// Elided or already-handled runes must report not-covered.
	for _, r := range []rune{
		'A', // ASCII
		'é', // Latin + diacritic (fold map handles it)
		'5', // digit
		'─', // box drawing (decorative block, elided)
		'⠁', // braille (elided)
		'日', // CJK Han (elided)
		'́', // combining acute (elided)
	} {
		_, ok := Word(r)
		assert.Falsef(t, ok, "Word(%q) should not be covered", r)
	}
}

// TestTableInvariants checks the generated tables are internally well-formed.
func TestTableInvariants(t *testing.T) {
	t.Parallel()

	n := len(nameWordID)
	numWords := len(wordOffLo) - 1

	// runStart strictly ascending; runFirstIndex strictly ascending starting at 0 with an N sentinel.
	require.Equal(t, len(runStart)+1, len(runFirstIndex), "runFirstIndex must have a trailing sentinel")
	require.Equal(t, uint32(0), runFirstIndex[0])
	require.Equal(t, uint32(n), runFirstIndex[len(runFirstIndex)-1], "sentinel must equal rune count") //nolint:gosec // in this case int is small and fits in uint32 (no overflow)
	for i := 1; i < len(runStart); i++ {
		require.Lessf(t, runStart[i-1], runStart[i], "runStart not ascending at %d", i)
		require.Lessf(t, runFirstIndex[i-1], runFirstIndex[i], "runFirstIndex not ascending at %d", i)
	}

	// Offsets (via the 18-bit sidecar) non-decreasing, first 0, last == blob length.
	require.Equal(t, uint32(0), offset18(0))
	require.Equal(t, uint32(len(wordBlob)), offset18(uint16(numWords))) //nolint:gosec // in this case int is small and fits in uint32 (no overflow)
	for id := 1; id <= numWords; id++ {
		require.LessOrEqualf(t, offset18(uint16(id-1)), offset18(uint16(id)), "offsets decrease at id %d", id)
	}

	// Every word id is in range and every word is non-empty.
	for i, id := range nameWordID {
		require.Lessf(t, int(id), numWords, "word id out of range at %d", i)
		lo, hi := offset18(id), offset18(id+1)
		require.Lessf(t, lo, hi, "empty word for id %d (rune index %d)", id, i)
	}
}

// TestCoverageRoundTrip walks every covered rune through the interval encoding and confirms Word() reconstructs the
// same word the raw arrays hold, and that inter-run gap runes miss.
func TestCoverageRoundTrip(t *testing.T) {
	t.Parallel()

	for i := 0; i < len(runStart); i++ {
		base := runStart[i]                            // codepoint of the run's first rune
		count := runFirstIndex[i+1] - runFirstIndex[i] // number of runes in the run
		for k := uint32(0); k < count; k++ {
			r := codePoint(t, base+k)
			id := nameWordID[runFirstIndex[i]+k]
			want := wordBlob[offset18(id):offset18(id+1)]

			got, ok := Word(r)
			require.Truef(t, ok, "Word(U+%04X) missing (run %d, +%d)", r, i, k)
			require.Equalf(t, want, got, "Word(U+%04X)", r)
		}

		// The codepoint just past this run's end (before the next run starts) must not be covered, unless it is the next
		// run's start.
		gap := base + count
		if i+1 < len(runStart) && gap < runStart[i+1] {
			_, ok := Word(codePoint(t, gap))
			require.Falsef(t, ok, "gap rune U+%04X after run %d should miss", gap, i)
		}
	}
}
