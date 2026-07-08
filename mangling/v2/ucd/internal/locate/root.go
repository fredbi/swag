package locate

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
	"unicode"
)

// Version describes one shipped UCD dataset and the Go toolchain baseline it targets.
//
// Each generated table comes in one flavor per Version, guarded by a //go:build constraint so exactly one links.
type Version struct {
	UCD   string // Unicode version, e.g. "15.0.0" — also the generated-file suffix (tables15.0.0.go)
	Dir   string // subfolder under the ucd root holding the extracts, e.g. "v15"
	MinGo string // lowest Go build tag this dataset serves ("go1.27"); "" for the baseline
}

// Versions lists every shipped dataset, ordered by increasing MinGo (baseline first).
//
// To add a version: append an entry here, add one //go:generate line per generator, then run `go generate ./...`.
// Build constraints are derived from adjacent MinGo bounds (see [BuildConstraint]), so appending a version
// automatically tightens the previous top entry's upper bound on the next regen.
var Versions = []Version{
	{UCD: "15.0.0", Dir: "v15", MinGo: ""},
	{UCD: "17.0.0", Dir: "v17", MinGo: "go1.27"},
}

// Lookup returns the Version whose UCD string matches, along with its index in [Versions].
func Lookup(ucd string) (Version, int, bool) {
	for i, v := range Versions {
		if v.UCD == ucd {
			return v, i, true
		}
	}

	return Version{}, -1, false
}

// BuildConstraint returns the //go:build expression (without the "//go:build " prefix) for the version at index i.
//
// It is bounded below by its own MinGo and above by the next version's MinGo:
//   - baseline (no lower bound): "!go1.27";
//   - top (no upper bound): "go1.27";
//   - a middle version: "go1.27 && !go1.28";
//   - a lone version: "" (no constraint).
func BuildConstraint(i int) string {
	lower := Versions[i].MinGo

	var upper string
	if i+1 < len(Versions) {
		upper = Versions[i+1].MinGo
	}

	switch {
	case lower == "" && upper == "":
		return ""
	case lower == "":
		return "!" + upper
	case upper == "":
		return lower
	default:
		return lower + " && !" + upper
	}
}

// Resolve maps a UCD version string to the input directory, its //go:build constraint and the filename suffix.
//
// rootOverride, when non-empty, replaces the git-root-derived UCD root (handy for tests); otherwise [UCDRoot] is used.
func Resolve(ucd, rootOverride string) (dir, buildTag, suffix string, err error) {
	v, i, ok := Lookup(ucd)
	if !ok {
		return "", "", "", fmt.Errorf("unknown UCD version %q (known: %v)", ucd, knownVersions())
	}

	root := rootOverride
	if root == "" {
		root, err = UCDRoot()
		if err != nil {
			return "", "", "", err
		}
	}

	return filepath.Join(root, v.Dir), BuildConstraint(i), v.UCD, nil
}

func knownVersions() []string {
	out := make([]string, len(Versions))
	for i, v := range Versions {
		out[i] = v.UCD
	}

	return out
}

// UCDRoot returns the directory holding the versioned UCD extracts (the parent of v15/, v17/, …), resolved from the
// repository git root.
func UCDRoot() (string, error) {
	const cmdTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	gitRoot, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return "", err
	}

	root := string(bytes.TrimRightFunc(gitRoot, func(r rune) bool {
		return r == '\n' || r == '\r' || unicode.IsSpace(r)
	}))

	return filepath.Join(root, "mangling", "v2", "ucd"), nil // TODO: temporary location
}
