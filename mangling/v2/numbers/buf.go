// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import "unsafe"

// buf is a minimal append-only byte sink shared by the string-returning verbalizers ([cardinal], [spellDecimal],
// [NumberMangler.NumberWords]) and the allocation-free [NumberMangler.AppendWords].
//
// It mirrors the subset of [strings.Builder]'s method set the verbalizer uses (WriteString / WriteByte / Write / Grow),
// so those functions are agnostic to whether they build a fresh string or append into a caller-owned, poolable buffer.
type buf struct{ b []byte }

func (w *buf) WriteString(s string) (int, error) { w.b = append(w.b, s...); return len(s), nil }
func (w *buf) WriteByte(c byte) error            { w.b = append(w.b, c); return nil }
func (w *buf) Write(p []byte) (int, error)       { w.b = append(w.b, p...); return len(p), nil }

// Grow ensures room for n more bytes, so a verbalized run that expands (e.g. "200" -> "two hundred") does not trigger a
// mid-build reallocation.
func (w *buf) Grow(n int) {
	if cap(w.b)-len(w.b) < n {
		nb := make([]byte, len(w.b), len(w.b)+n)
		copy(nb, w.b)
		w.b = nb
	}
}

// unsafeStr returns b as a string without copying.
//
// Safe only for a freshly built buffer that is not mutated afterwards.
//
// This is the same trick [strings.Builder.String] uses internally.
func unsafeStr(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	return unsafe.String(&b[0], len(b))
}
