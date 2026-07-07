// Package ucd hosts the Unicode Character Database (UCD) source extracts and the code generators that turn them into
// the static lookup tables the mangling packages rely on.
//
// It is a separate module (github.com/go-openapi/swag/mangling/v2/ucd) on purpose: the generator tooling and its data
// live here, so nothing this module depends on leaks into the mangling module.
// The generated tables (runewords/tables.go, numbers/numerals.go) are plain, dependency-free Go source — the mangling
// module never imports this one.
//
// # Layout
//
//	ucd/
//	  v15/                          versioned UCD extracts (one directory per Unicode version)
//	    DerivedName.txt             character names            -> runewords/tables.go
//	    DerivedNumericValues.txt    numeric values (No/Nl)     -> numbers/numerals.go
//	    emoji-data.txt              Extended_Pictographic gate -> runewords/tables.go
//	  cmd/
//	    gen_runewords/              builds the compact rune -> word table
//	    gen_numerals/               builds the rune -> numeric value table
//	  internal/locate/             resolves the active UCD data directory from the repo git root
//
// # Regenerating
//
// Each consuming package carries a go:generate directive pointing at the matching command.
// Regenerate everything from the module root:
//
//	go generate ./...
//
// or a single table from its own package (for example, from runewords/):
//
//	go generate
//
// The generators are idempotent: with unchanged data and unchanged generator code they emit byte-identical tables, so a
// clean tree after go generate is the expected state.
//
// # Bumping the Unicode version
//
// Drop the new extracts under a fresh versioned directory (e.g. ucd/v17/), point defaultUCDVersion in internal/locate
// at it, and re-run go generate.
// Keeping versions side by side makes a bump reviewable as a data diff plus a regenerated-table diff, rather than an
// in-place overwrite.
package ucd
