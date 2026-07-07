# mangling

The `mangling` package exposes utilities to turn strings into valid `go` identifiers.

It also allows common recasing operations like camel-casing, kebab-casing, etc.

It supersedes `github.com/go-openapi/swag/mangling`, providing an equivalent functionality,
with a more stricter, more robust yet faster implementation.

## Main features

* `Mangler`
* `GoMangler`
* `numbers/NumberMangler`

## Asciify: Verbalization & romanisation

Numbers

Latin with diacritics

Non-latin

Emojis & others

Not supported for now:
* CJK (unicode points for East-Asian languages).
* graphemes

## `GoMangler` API

The contract: the go mangler turns any string into a valid go identifier.

### `GoIdentExported`, `GoIdentUnexported`: go identifiers rules

go compiler constraints:

A `go` identifier (exported or unexported) must start with a letter.
The remainder is made of letters or unicode decimal digits.

Letters are unicode letters or "_". 

If the initial letter is upper-case, the identifier is exported ("public").

Identifiers must not conflict with a reserved word of the go language.

usage enforced by linters

* avoidance of "_", camel-case or pascal-case are considered idiomatic
* initialisms (`revive`)
* unicode is accepted, but ASCII-only identifiers are usually preferred (`gosmopolitan`)
* identifiers should not shadow a go builtin function

### `File`: go file names

### `Package`, `Module`: go package and module names

### `GoConstName`: identifiers for enum values

Differences with `GoIdentExported`.

## `Mangler` API

## Differences with `go-openapi/swag/mangling@v0.x`

## Tests

The go mangler is fuzzed with the objective of producing a valid go identifier against all-weather input.

## Performances

A typical mangling operation takes about 1,000 ns.

All mangling methods scale linearly with the number of tokens in the input string (~< 200-300 ns/token),
and perform zero internal allocation (only the returned string is allocated).
