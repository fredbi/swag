// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"sync"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// TestManglerConcurrency asserts that both Mangler and GoMangler are fully reentrant.
//
// A single constructed instance (value and pointer forms) is shared across many goroutines,
// each hammering every output method.
//
// It guards two things at once:
//
//   - data races on the shared read-only dictionaries and the package-level token sync.Pool (run with -race),
//   - output corruption (every concurrent result must equal the golden value computed single-threaded beforehand).
func TestManglerConcurrency(t *testing.T) {
	t.Parallel()

	g := MakeGoMangler()                     // value form, shared
	gp := NewGoMangler()                     // pointer form, shared
	m := MakeMangler(WithASCIIFolding(true)) // base mangler, shared

	// Varied inputs (lengths, scripts, numbers, symbols) to churn the token pool through many shapes.
	checks := []struct {
		name string
		fn   func() string
	}{
		{"IdentExported", func() string { return g.IdentExported("http server id") }},
		{"IdentUnexported", func() string { return g.IdentUnexported("type getHTTP thing") }},
		{"ConstName", func() string { return g.ConstName("area ½ over 200") }},
		{"Package", func() string { s, p := g.Package("github.com/x/@alpha-beta"); return s + "|" + p }},
		{"Module", func() string { return g.Module("foo/bar/v2") }},
		{"File", func() string { return g.File("IPv4Config.json") }},
		{"Ident(ptr)", func() string { return gp.IdentExported("json rpc payload") }},
		{"ConstName(ptr)", func() string { return gp.ConstName("status 404 café") }},
		{"Camelize", func() string { return m.Camelize("café ☯ 日本 value") }},
		{"Snakize", func() string { return m.Snakize("HTTPServerConfig") }},
		{"Pascalize", func() string { return m.Pascalize("some-kebab-name") }},
		{"AllCaps", func() string { return m.AllCaps("mixed Case words") }},
		{"ToASCII", func() string { return ToASCII("½ π 😀 café") }},
	}

	// Golden values: computed before any goroutine starts, so they are race-free references.
	want := make([]string, len(checks))
	for i, c := range checks {
		want[i] = c.fn()
	}

	const (
		goroutines = 64
		iterations = 300
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				for i, c := range checks {
					if got := c.fn(); got != want[i] {
						assert.EqualTf(t, want[i], got, "concurrent %s produced a corrupt result", c.name)

						return // one report per goroutine is enough
					}
				}
			}
		}()
	}
	wg.Wait()
}
