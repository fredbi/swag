// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runewords

import "sort"

// Word returns the distinctive lowercase word(s) for rune r.
//
// e.g. 'α' -> "alpha", '😀' -> "grinning face", and whether r is covered.
//
// Covered runes exclude everything the mangler already handles (ASCII, Latin+diacritics, digits) or elides (combining
// marks, controls, separators, CJK, Hangul).
//
// Callers re-segment the result (following case breaks).
//
// The covered set is interval-encoded (maximal ranges of consecutive codepoints): a single binary search over runStart
// locates the range, then the global rune index is pure arithmetic — no linear scan.
//
// Offsets into wordBlob are 18-bit (uint16 low + 2-bit high sidecar).
func Word(r rune) (string, bool) {
	u := uint32(r) //nolint:gosec // false positive: rune aliases to int32, so it's okay to consider the result unsigned

	// Locate the maximal run whose start is <= r (largest runStart[i] <= u).
	i := sort.Search(len(runStart), func(i int) bool { return runStart[i] > u })
	if i == 0 {
		return "", false // r precedes the first covered run
	}
	i--

	// Position within the run; reject r if it falls in the gap after the run's end.
	pos := runFirstIndex[i] + (u - runStart[i])
	if pos >= runFirstIndex[i+1] {
		return "", false
	}

	id := nameWordID[pos]
	return wordBlob[offset18(id):offset18(id+1)], true
}

// offset18 reconstructs the 18-bit blob offset for word id from the uint16 low array and the 2-bit high sidecar (packed
// 4 entries per byte).
func offset18(id uint16) uint32 {
	const (
		sidecarMask = 3
		hiExtraBits = 2
		hiMask      = 0x3
	)
	hi := uint32(wordOffHi[id>>hiExtraBits]>>(hiExtraBits*(id&sidecarMask))) & hiMask
	return hi<<16 | uint32(wordOffLo[id])
}
