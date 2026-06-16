package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveSafePath resolves and validates a user-supplied file path against
// an optional working directory (root). It prevents path traversal attacks
// by ensuring the resolved absolute path stays within root.
//
// Rules:
//   - If root is empty, the process working directory is used as the jail.
//   - Relative paths are resolved relative to root.
//   - Absolute paths must reside inside root.
//   - Symlinks are checked and must NOT point outside the jail.
func resolveSafePath(userPath string, root string) (string, error) {
	if userPath == "" {
		return "", fmt.Errorf("empty path")
	}

	// Normalize separators.
	userPath = filepath.FromSlash(userPath)

	// Determine the jail root.
	jail := root
	if jail == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot determine working directory: %w", err)
		}
		jail = wd
	}
	jail, err := filepath.Abs(jail)
	if err != nil {
		return "", fmt.Errorf("cannot resolve root: %w", err)
	}
	jail = filepath.Clean(jail)

	// Resolve the user path against the jail.
	var abs string
	if filepath.IsAbs(userPath) {
		abs = filepath.Clean(userPath)
	} else {
		abs = filepath.Clean(filepath.Join(jail, userPath))
	}

	// Check that abs is within jail (or equal to jail).
	if !isPathWithin(abs, jail) {
		return "", fmt.Errorf("path %q escapes the working directory", userPath)
	}

	// Resolve symlinks and re-validate the final real path.
	realPath, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			// Path doesn't exist yet (e.g. write_file to new path).
			// Validate the parent directory instead.
			parent := filepath.Dir(abs)
			realParent, err2 := filepath.EvalSymlinks(parent)
			if err2 == nil && !isPathWithin(realParent, jail) {
				return "", fmt.Errorf("path %q escapes the working directory via symlink", userPath)
			}
			return abs, nil
		}
		return "", fmt.Errorf("cannot resolve symlinks for %q: %w", userPath, err)
	}
	if !isPathWithin(realPath, jail) {
		return "", fmt.Errorf("symlink target for %q escapes the working directory", userPath)
	}

	return abs, nil
}

// isPathWithin checks that path is equal to jail or a descendant of it.
func isPathWithin(path, jail string) bool {
	return strings.HasPrefix(path, jail+string(filepath.Separator)) || path == jail
}
