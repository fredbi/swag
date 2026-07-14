// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package ucd hosts the Unicode Character Database (UCD) source extracts and the code generators that turn them into
// the static lookup tables the mangling packages rely on.
//
// It is a separate module (github.com/go-openapi/swag/mangling/v2/ucd) on purpose: the generator tooling and its data
// live here, so nothing this module depends on leaks into the mangling module.
// The generated tables are plain, dependency-free Go source — the mangling module never imports this one.
//
// Each table is generated once per Unicode version, and the flavors are selected at compile time by a //go:build guard
// so exactly one links: for example runewords/tables15.0.0.go (//go:build !go1.27) and tables17.0.0.go (//go:build
// go1.27). The set of versions and their Go baselines lives in internal/locate (internal/locate.Versions); the build tags are
// derived from adjacent baselines by internal/locate.BuildConstraint.
//
// # Layout
//
//	ucd/
//	  v15/ v17/                     versioned UCD extracts (one directory per Unicode version)
//	    DerivedName.txt             character names            -> runewords/tables<ver>.go, asciifold_table<ver>.go
//	    DerivedNumericValues.txt    numeric values (No/Nl)     -> numbers/numerals<ver>.go
//	    emoji-data.txt              Extended_Pictographic gate -> runewords/tables<ver>.go
//	  cmd/
//	    gen_runewords/              builds the compact rune -> word table
//	    gen_numerals/               builds the rune -> numeric value table
//	    gen_asciifold/              builds the Latin diacritic-fold table
//	  internal/locate/             the version registry + the UCD root resolved from the repo git root
//
// Each generator takes [package [outbase [version [ucd-root]]]] and emits <outbase><version>.go with the derived
// //go:build tag; the go:generate directive in each consuming package is repeated once per version.
//
// # Regenerating
//
// Regenerate everything from the module root, or a single table from its own package:
//
//	go generate ./...
//	go generate            # e.g. from runewords/
//
// The generators are idempotent: with unchanged data and generator code they emit byte-identical tables, so a clean
// tree after go generate is the expected state.
//
// NOTE: gen_runewords classifies runes with the *toolchain's* unicode tables (unicode.Is), so a version must be
// regenerated under a Go toolchain whose Unicode version matches — the go1.27 flavor requires go1.27+ (otherwise runes
// newly assigned in Unicode 17 are seen as unassigned and dropped). gen_asciifold (name-driven) and gen_numerals
// (category read from the data file) are toolchain-independent.
//
// # Bumping the Unicode version
//
// Drop the new extracts under a fresh versioned directory (e.g. ucd/v19/), append an internal/locate.Version entry (with its
// Go baseline) to internal/locate.Versions, add one //go:generate line per consuming package, and re-run go generate under the
// matching toolchain. Keeping versions side by side makes a bump reviewable as a data diff plus a regenerated-table
// diff, rather than an in-place overwrite.
package ucd
