# Plan: Sandboxed local loading via `WithRoot` (GHSA-v2xp-g8xf-22pf)

## Problem

`loading.LoadFromFileOrHTTP` resolves a `file://` URI (or a bare path) to a local
filesystem path by stripping the `file://` prefix and handing the result straight to
`os.ReadFile`, with no confinement (`loading/loading.go:69-78`, default `ReadFileFunc`
returns `os.ReadFile` in `loading/options.go:35-41`).

An application that forwards untrusted input to `LoadFromFileOrHTTP` — or to downstream
`loads.Spec()` / `loads.JSONSpec()` — can therefore read any file the process can read
(`file:///etc/passwd`, `/root/.aws/credentials`, …). CWE-22 / CWE-73.

Two facts make this worth a first-class fix rather than a "works as intended" dismissal:

1. **Our preprocessing defeats downstream input validation.** A caller that defensively
   rejects paths starting with `/` is still bypassed by `file:///etc/passwd`, because we
   strip the scheme *after* their check and re-introduce the absolute path. `url.PathUnescape`
   compounds this (percent-encoded separators).
2. **The existing safe alternative is under-powered.** `WithFS(os.DirFS(dir))` blocks
   traversal (`fs.ValidPath` rejects a leading `/` and any `..` element), but plain
   `os.DirFS` is explicitly **not** symlink-safe: a symlink inside the root pointing at
   `/etc/passwd` escapes. This module is already on Go 1.25, so `os.Root` (Go 1.24+) gives
   us a symlink-escape-resistant sandbox for free.

## Decision (agreed)

- **Posture: opt-in hardening, non-breaking.** Default behavior is unchanged; no major
  version bump of the `loading` module. Callers opt into confinement.
- **Mechanism: a single new option, `WithRoot(dir string)`, built on `os.Root`.**
  Not in scope for this change: changing the default, path-policy validators, deny-by-default
  for `file://` / absolute paths. (Can revisit later if the GHSA reviewer pushes for
  default-deny.)

## Why `os.Root`

`os.OpenRoot(dir) (*os.Root, error)` returns a directory handle. `(*os.Root).FS()` returns
an `fs.FS` that already implements `fs.ReadFileFS` (verified on the toolchain). Methods on a
`Root`:

- reject any name that escapes the root (absolute paths, `..` past the root);
- follow internal symlinks but reject symlinks that resolve outside the root, and reject
  absolute symlinks;
- are concurrency-safe.

Honest limitations to document (from the stdlib docs): `os.Root` does **not** prohibit
traversal of mount/bind boundaries, `/proc` special files, or Unix device files. It is a
path-confinement sandbox, not a chroot/jail. It fully addresses the reported vector
(absolute-path and symlink-based arbitrary read confined to a chosen directory).

## Approach

### 1. Add the `WithRoot` option (`loading/options.go`)

`WithRoot` confines all local reads to `dir`, resolving every requested path relative to it
and rejecting anything that escapes. Reads route through `os.Root`, so they are
symlink-escape-resistant — strictly safer than `WithFS(os.DirFS(dir))`.

`os.OpenRoot` can fail (missing dir, not a directory) and the `Option` signature
(`func(*options)`) cannot return an error. **Design (decided): open the root lazily, per
read, and `Close` it** — this surfaces the open error naturally at load time and never leaks
a file descriptor (no reliance on the `*os.Root` finalizer). Spec loads are infrequent and
typically one-shot, so the per-read `OpenRoot`/`Close` cost is negligible.

Reads go through `(*os.Root).ReadFile(name)` (not `r.FS().ReadFile`). Verified behavior on
the toolchain: it accepts in-root relative paths including `./x` and internal `sub/../x`,
and **rejects** absolute paths (`/etc/passwd`), `..` traversal, and both relative and
absolute symlink escapes — the symlink case being exactly what `os.DirFS` fails to block.

```go
// fileOptions gains a rooted-loader marker.
type fileOptions struct {
    fs   fs.ReadFileFS
    root string // when non-empty, confine local reads to this directory via os.Root
}

// WithRoot confines local file loading to dir, resolving requested paths relative to it
// and rejecting any path (including via symlink) that escapes dir. Built on os.Root, so it
// resists symlink-based escapes that os.DirFS does not.
//
// WithRoot is the recommended way to load specs from a location derived from untrusted
// input. It does not affect remote (http/https) loading. WithRoot and WithFS are
// mutually exclusive: the last one applied wins.
//
// Note: os.Root confines path resolution but does not protect against mount/bind
// boundaries, /proc special files, or device files. Point WithRoot at a directory that
// contains only the specs you intend to expose.
func WithRoot(dir string) Option {
    return func(o *options) { o.root = dir; o.fs = nil } // last-wins vs WithFS
}

func (fo fileOptions) ReadFileFunc() func(string) ([]byte, error) {
    if fo.root != "" {
        root := fo.root
        return func(name string) ([]byte, error) {
            r, err := os.OpenRoot(root)
            if err != nil {
                return nil, errors.Join(err, ErrLoader) // wrapped per decision (2)
            }
            defer func() { _ = r.Close() }()
            return r.ReadFile(name)
        }
    }
    if fo.fs == nil {
        return os.ReadFile
    }
    return fo.fs.ReadFile
}
```

`WithFS` symmetrically clears `o.root` so last-wins holds in both orders.

### 2. Confirm `LoadStrategy` needs no security change

For `file:///etc/passwd`, the existing flow strips to `/etc/passwd` and calls
`local("/etc/passwd")`. With `WithRoot`, that reaches `r.FS().ReadFile("/etc/passwd")`,
which fails `fs.ValidPath` (leading slash) → rejected. A symlink inside the root is rejected
by `os.Root` itself. So **no change to `loading.go` is required for the fix.** A legitimate
nested read (`specs/api.yaml`) resolves under the root as intended.

### 3. Windows nested-path slashing — fix (decided: 3b)

On non-Windows (the security-relevant platform for the report) `filepath.FromSlash` is a
no-op, so paths reach the fs loader as valid slash paths. On **Windows**, the existing
`LoadStrategy` applies `filepath.FromSlash` to non-`embed.FS` loaders, turning
`specs/api.yaml` into `specs\api.yaml`, which is **not** a valid `fs.FS` path. This already
affects `WithFS` with multi-segment paths on Windows (the current test only exercises a
single-segment MapFS name, so it never trips). `WithRoot` would inherit it.

**Fix:** treat *any* fs-backed loader as forward-slash, like `embed.FS`. Introduce
`isFSBacked := o.fs != nil || o.root != ""` and use it (instead of `isEmbedFS`) in the guard
that bypasses the Windows-native `file://` UNC branch — that branch is only meaningful for
the `os.ReadFile` default loader. Within the fs-backed branch:

- **`embed.FS`** keeps its existing rule exactly: separator is always `/` even on Windows,
  with the leading `./` strip (`strings.TrimLeft(filepath.ToSlash(cpth), "./")`).
- **other fs.FS / `WithRoot`**: `filepath.ToSlash(cpth)` only — do **not** strip leading
  `./` or `..`; let `os.Root` / `fs.ValidPath` reject escapes (we want rejection, not silent
  rewriting, for a security boundary).
- **default `os.ReadFile`**: unchanged, `filepath.FromSlash(cpth)`.

This fixes `WithFS` on Windows too and is not a regression (that path is already broken).
`os.Root.ReadFile` accepts `/`-separated paths on all platforms, so `WithRoot` works
cross-platform through this branch.

### 4. Documentation

- Godoc on `WithRoot` (above), explicitly recommending it for untrusted input and stating
  the `os.Root` limitations honestly.
- A short **Security** section in `loading/doc.go` (or a `SECURITY` note) describing the
  default permissive behavior and pointing untrusted-input callers to `WithRoot`.
- Coordinate a one-line note for downstream `loads` docs (separate repo) so the GHSA can
  reference a concrete remediation.

## Tests (`loading/loading_test.go` + a new focused test file)

All under `t.Run`, table-driven where natural. Use `t.TempDir()` for the root.

- **Confined happy path:** `WithRoot(tmp)` + `LoadFromFileOrHTTP("api.yaml")` and a nested
  `sub/api.yaml` both load.
- **Absolute path rejected:** `LoadFromFileOrHTTP("file:///etc/passwd", WithRoot(tmp))`
  returns an error and does **not** read outside the root. Assert no bytes leak.
- **Traversal rejected:** `../outside.yaml` (place a file outside `tmp`) → error.
- **Symlink escape rejected:** create `tmp/link -> /etc/passwd` (or to a file outside `tmp`),
  load `link` → error. This is the case `os.DirFS` would *fail* to block — assert it to lock
  in the `os.Root` benefit. Guard with `runtime.GOOS != "windows"` if symlink creation is
  unreliable there, or use `testing` helpers / skip on platforms without symlink privilege.
- **Bad root:** `WithRoot("/nonexistent")` → error surfaced at load time, wrapped so
  `errors.Is(err, ErrLoader)` holds (if we adopt wrapping).
- **Default unchanged (regression guard):** existing `os.ReadFile` behavior without
  `WithRoot` still reads an absolute path, proving non-breaking.
- If we take 3(b): a Windows-tagged (or slash-normalization unit) test for multi-segment
  relative paths through an fs/root loader.

Run: `go test ./loading/...`, then `go test work ./...`; `golangci-lint run`.

## Out of scope (explicitly)

- Changing the default posture / deny-by-default for `file://` or absolute paths.
- Path-policy validator options (`WithoutFileURI`, `WithDenyAbsolute`, …).
- Strengthening/redocumenting `WithFS` beyond what 3(b) incidentally improves.
- Any change in the `loads` repo (tracked separately; only doc coordination here).

## Resolved decisions

1. `WithRoot` × `WithFS`: **last-wins** (each option clears the other; documented).
2. **Wrap** the `os.OpenRoot` error with `ErrLoader` (`errors.Join`) for `errors.Is`.
3. Windows slashing: **fix in `LoadStrategy` (3b)**, preserving `embed.FS`'s always-`/` rule.
4. **Per-read `OpenRoot`/`Close`** — no fd leak, no finalizer reliance.
