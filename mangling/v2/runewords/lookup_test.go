package runewords

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

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
	require.Equal(t, uint32(n), runFirstIndex[len(runFirstIndex)-1], "sentinel must equal rune count")
	for i := 1; i < len(runStart); i++ {
		require.Lessf(t, runStart[i-1], runStart[i], "runStart not ascending at %d", i)
		require.Lessf(t, runFirstIndex[i-1], runFirstIndex[i], "runFirstIndex not ascending at %d", i)
	}

	// Offsets (via the 18-bit sidecar) non-decreasing, first 0, last == blob length.
	require.Equal(t, uint32(0), offset18(0))
	require.Equal(t, uint32(len(wordBlob)), offset18(uint16(numWords)))
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

// TestCoverageRoundTrip walks every covered rune through the interval encoding and confirms
// Word() reconstructs the same word the raw arrays hold, and that inter-run gap runes miss.
func TestCoverageRoundTrip(t *testing.T) {
	t.Parallel()

	for i := 0; i < len(runStart); i++ {
		start := rune(runStart[i])
		count := int(runFirstIndex[i+1] - runFirstIndex[i])
		for k := 0; k < count; k++ {
			r := start + rune(k)
			pos := runFirstIndex[i] + uint32(k)
			id := nameWordID[pos]
			want := wordBlob[offset18(id):offset18(id+1)]

			got, ok := Word(r)
			require.Truef(t, ok, "Word(U+%04X) missing (run %d, +%d)", r, i, k)
			require.Equalf(t, want, got, "Word(U+%04X)", r)
		}

		// The rune just past this run's end (before the next run starts) must not be covered,
		// unless it is the next run's start.
		gap := start + rune(count)
		if i+1 < len(runStart) && gap < rune(runStart[i+1]) {
			_, ok := Word(gap)
			require.Falsef(t, ok, "gap rune U+%04X after run %d should miss", gap, i)
		}
	}
}
