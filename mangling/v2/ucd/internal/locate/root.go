package locate

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"time"
	"unicode"
)

const defaultUCDVersion = "v15"

func UCD() (string, error) {
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

	// return filepath.Join(root, "ucd", defaultUCDVersion), nil
	return filepath.Join(root, "mangling", "v2", "ucd", defaultUCDVersion), nil // TODO: temporary location
}
