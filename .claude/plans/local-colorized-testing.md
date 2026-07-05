# Plan: Local Colorized Test Output

## Objective

Enable colorized testify output for local development without altering
the project's existing dependencies or test behavior in CI.

`github.com/go-openapi/testify/v2` supports colorized output via
a separate enabler module (`enable/colors/v2`). The goal is to make
this easy to opt into locally.

## Approach

Add a thin `testing/colors` module to the workspace. This module's only
purpose is a blank import of the testify colors enabler.

### 1. Create the module

```
testing/colors/
  go.mod       # module github.com/go-openapi/swag/testing/colors
  colors.go    # blank import: _ "github.com/go-openapi/testify/enable/colors/v2"
```

The `.go` file should be a `_test` package or a `main`-like stub — whichever
keeps `go vet` happy.

### 2. Register in go.work

```
use ./testing/colors
```

This makes the module part of the workspace. `go test work ./...` will
resolve it alongside everything else.

### 3. Update release CI

The bump-release workflow (mono-repo variant) tags and releases every
module listed in `go.work`. `testing/colors` must be excluded because
it is not a published module.

Filter it out in the release workflow so it is never tagged or released.

### 4. Document usage

In the README or CONTRIBUTING guide, note that running:

```sh
go test work ./...
```

automatically enables colorized output because the workspace includes
`testing/colors`.

For users who don't use the workspace (e.g. `go test ./conv/...`),
colors won't be active — which is fine, no action needed.

## Notes

- The colors enabler dependency is lightweight; CI downloading it is acceptable.
- CI will run tests for `testing/colors` — this is harmless.
- No build tags needed: the module is always compiled, the dependency is always resolved.
