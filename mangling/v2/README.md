# mangling

The `mangling` package exposes utilities to turn strings into valid `go` identifiers.

It also allows common recasing operations like camel-casing, kebab-casing, etc.

It supersedes `github.com/go-openapi/swag/mangling`, providing an equivalent functionality,
with a more robust and faster implementation.

## go identifiers rules

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

## go file names

## go package and module names

## Identifiers for enum values
