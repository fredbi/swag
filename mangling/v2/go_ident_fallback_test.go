// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// TestGoIdentFallback covers the contract that Go identifier producers never emit an empty identifier.
//
// Input that reduces to nothing (empty, all-separators, or all-elided runes) yields a fallback word, cased per target.
//
// Package/Module are exempt (they carry a dir prefix), and the base Mangler is not.
func TestGoIdentFallback(t *testing.T) {
	t.Parallel()

	// each of these reduces to nothing after segmentation / ASCII folding
	// (note: "---" is no longer here — "--" now verbalizes as "decrement")
	emptyish := []string{"", "___", "   ", "日本" /* CJK, folded away */, "_", "."}

	t.Run("default fallback, cased per target", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler()
		for _, in := range emptyish {
			if in == "." {
				continue // "." → "Dot" (a symbol with a name), not empty — covered elsewhere
			}
			assert.EqualTf(t, "Empty", g.IdentExported(in), "IdentExported(%q)", in)
			assert.EqualTf(t, "empty", g.IdentUnexported(in), "IdentUnexported(%q)", in)
			assert.EqualTf(t, "Empty", g.ConstName(in), "ConstName(%q)", in)
			assert.EqualTf(t, "empty", g.File(in), "File(%q)", in)
		}
	})

	t.Run("File keeps dir and extension around the fallback stem", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler()
		assert.EqualT(t, "sub/empty.go", g.File("sub/___.go"))
		assert.EqualT(t, "a/b/empty", g.File("a/b/___"))
	})

	t.Run("Package and Module stay empty-allowed (dir-only)", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler()
		short, pkg := g.Package("___")
		assert.EqualT(t, "", short)
		assert.EqualT(t, "", pkg)
		assert.EqualT(t, "sub/", g.Module("sub/___"))
	})

	t.Run("configured fallback is itself sanitized and cased", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler(WithGoIdentFallback("unknown value"))
		assert.EqualT(t, "UnknownValue", g.IdentExported("___"))
		assert.EqualT(t, "unknownValue", g.IdentUnexported("___"))
		assert.EqualT(t, "unknown_value", g.File("___"))
	})

	t.Run("a fallback that itself reduces to nothing guards to the built-in default", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler(WithGoIdentFallback("___"))
		assert.EqualT(t, "Empty", g.IdentExported("___"))
		assert.EqualT(t, "empty", g.IdentUnexported(""))
	})

	t.Run("non-empty input is unaffected", func(t *testing.T) {
		t.Parallel()
		g := MakeGoMangler()
		assert.EqualT(t, "HelloWorld", g.IdentExported("hello world"))
		assert.EqualT(t, "AtHashDollar", g.IdentExported("@#$")) // symbols name themselves, not empty
	})

	t.Run("the base Mangler has no such contract (may return empty)", func(t *testing.T) {
		t.Parallel()
		m := MakeMangler(WithASCIIFolding(true))
		assert.EqualT(t, "", m.Camelize("___"))
		assert.EqualT(t, "", m.Pascalize("日本")) // elided CJK runes
	})
}
