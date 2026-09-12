// Package safepath builds filesystem paths from untrusted input, such as
// user-supplied course names, without letting that input escape the directory
// it is supposed to live in.
package safepath

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrOutsideRoot is returned when a joined path would leave its root directory.
var ErrOutsideRoot = errors.New("resolved path escapes its root directory")

// replacement is substituted for characters that would otherwise change the
// structure of a path.
const replacement = "_"

// Component sanitizes s so it can be used as a single path segment. Separators
// are replaced rather than stripped, so that names which legitimately contain
// them (e.g. the course "Analysis I/II") stay readable instead of collapsing
// into each other. Segments that only have meaning to the filesystem ("." and
// "..") and control characters are replaced as well. An input that carries no
// usable characters yields a single replacement, never an empty string, so the
// caller always gets a real directory level.
func Component(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == 0:
			return '_'
		case r < 0x20 || r == 0x7f:
			return '_'
		default:
			return r
		}
	}, s)

	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == ".." {
		return replacement
	}
	return s
}

// JoinInRoot joins the given elements onto root, sanitizing each one, and
// verifies that the result is still inside root. The containment check is a
// backstop: Component should already make an escape impossible, but the two
// together mean a gap in one does not turn into a write outside the storage
// directory.
func JoinInRoot(root string, elems ...string) (string, error) {
	cleanRoot := filepath.Clean(root)

	sanitized := make([]string, 0, len(elems)+1)
	sanitized = append(sanitized, cleanRoot)
	for _, elem := range elems {
		sanitized = append(sanitized, Component(elem))
	}

	joined := filepath.Join(sanitized...)
	if !Contains(cleanRoot, joined) {
		return "", fmt.Errorf("%w: %s", ErrOutsideRoot, joined)
	}
	return joined, nil
}

// Contains reports whether path lies inside root. root itself counts as
// contained.
func Contains(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
