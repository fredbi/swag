// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

func TestNumberManglerNumberWords(t *testing.T) {
	t.Parallel()

	t.Run("with defaults", func(t *testing.T) {
		t.Parallel()

		m := MakeNumberMangler()
		for tc := range manglerDefaultCases() {
			assert.EqualTf(t, tc.out, m.NumberWords(tc.in), "NumberWords(%q)", tc.in)
		}
	})

	t.Run("with StripAnd", func(t *testing.T) {
		t.Parallel()

		m := MakeNumberMangler(WithNumberStripAnd(true))
		assert.EqualT(t, "one hundred twenty three", m.NumberWords("123"))
	})

	t.Run("with StripOne", func(t *testing.T) {
		t.Parallel()

		m := MakeNumberMangler(WithNumberStripOne(true))
		assert.EqualT(t, "hundred and tenth", m.NumberWords("100 and 0.1"))
	})

	t.Run("with special numbers", func(t *testing.T) {
		t.Parallel()

		m := MakeNumberMangler(WithSpecialNumbers(map[string]string{"3.1415": "pi", "2.718": "e"}))
		for tc := range manglerSpecialCases() {
			assert.EqualTf(t, tc.out, m.NumberWords(tc.in), "NumberWords(%q)", tc.in)
		}
	})

	t.Run("with thousands separators", func(t *testing.T) {
		t.Parallel()

		m := MakeNumberMangler()
		for tc := range manglerThousandsCases() {
			assert.EqualTf(t, tc.out, m.NumberWords(tc.in), "NumberWords(%q)", tc.in)
		}
	})
}

// Not parallel: the allocation-free subtest uses testing.AllocsPerRun, which reads process-wide memory stats and would
// be perturbed by concurrent tests.
func TestNumberManglerAppendWords(t *testing.T) {
	m := MakeNumberMangler()

	t.Run("matches NumberWords, appending after existing content", func(t *testing.T) {
		for _, seq := range []iter.Seq[manglerCase]{manglerDefaultCases(), manglerThousandsCases()} {
			for tc := range seq {
				got := string(m.AppendWords([]byte("<"), tc.in))
				assert.EqualTf(t, "<"+tc.out, got, "AppendWords(%q)", tc.in)
			}
		}
	})

	t.Run("with special numbers", func(t *testing.T) {
		ms := MakeNumberMangler(WithSpecialNumbers(map[string]string{"3.1415": "pi", "2.718": "e"}))
		for tc := range manglerSpecialCases() {
			assert.EqualTf(t, tc.out, string(ms.AppendWords(nil, tc.in)), "AppendWords(%q)", tc.in)
		}
	})

	t.Run("a reused buffer verbalizes allocation-free", func(t *testing.T) {
		const in = "order 200 status 404 progress 0.5" // no thousands separators (those need a strip copy)
		var scratch []byte
		scratch = m.AppendWords(scratch[:0], in) // warm: size the buffer once
		allocs := testing.AllocsPerRun(100, func() {
			scratch = m.AppendWords(scratch[:0], in)
		})
		assert.Truef(t, allocs == 0, "pooled AppendWords should not allocate, got %v", allocs)
	})
}

type manglerCase struct {
	in, out string
}

func manglerDefaultCases() iter.Seq[manglerCase] {
	return slices.Values([]manglerCase{
		{"", ""},
		{"no numbers here", "no numbers here"},
		{"123", "one hundred and twenty three"},
		{"10 11", "ten eleven"},
		{"11 and 12", "eleven and twelve"},
		{"level 0.25 here", "level one quarter here"},
		{"0.31456", "zero dot three one four five six"},
		{"1.5", "one dot five"},
		{"temperature -5 degrees", "temperature minus five degrees"},
		{"a-5", "a-five"}, // hyphen, not a sign
	})
}

func manglerSpecialCases() iter.Seq[manglerCase] {
	return slices.Values([]manglerCase{
		{"3.1415", "pi"},
		{"3.14159", "pi"}, // within tolerance of the registered value
		{"the value is 2.718 here", "the value is e here"},
		{"123", "one hundred and twenty three"}, // not special: normal cardinal
		{"3.5", "three dot five"},               // not close to any special
	})
}

func manglerThousandsCases() iter.Seq[manglerCase] {
	return slices.Values([]manglerCase{
		{"1 234", "one thousand two hundred and thirty four"},
		{"1,234", "one thousand two hundred and thirty four"},
		{"1_234", "one thousand two hundred and thirty four"},
		{"12 345", "twelve thousand three hundred and forty five"},
		{"1 2", "one two"},                           // not three digits: kept separate
		{"1;234", "one;two hundred and thirty four"}, // ";" is not a separator
		{"value 1 000 000 items", "value one million items"},
		{"1 234.5", "one thousand two hundred and thirty four dot five"},
	})
}
