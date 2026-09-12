package safepath

import (
	"path/filepath"
	"testing"
)

func TestComponent(t *testing.T) {
	cases := map[string]string{
		"Funktionale Programmierung (IN0003)": "Funktionale Programmierung (IN0003)",
		"Analysis I/II":                       "Analysis I_II",
		"../../escaped":                       ".._.._escaped",
		"..":                                  "_",
		".":                                   "_",
		"":                                    "_",
		"   ":                                 "_",
		"/":                                   "_",
		"a\\b":                                "a_b",
		"nul\x00byte":                         "nul_byte",
		"new\nline":                           "new_line",
	}
	for in, want := range cases {
		if got := Component(in); got != want {
			t.Errorf("Component(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComponentNeverEscapes(t *testing.T) {
	root := "/mass"
	for _, name := range []string{"../../escaped", "..", "/etc", "a/../../b", "....//"} {
		got := filepath.Join(root, Component(name))
		if !Contains(root, got) {
			t.Errorf("Component(%q) escaped root: %s", name, got)
		}
	}
}

func TestJoinInRoot(t *testing.T) {
	got, err := JoinInRoot("/mass", "../../escaped.2022", "files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "/mass/.._.._escaped.2022/files"; got != want {
		t.Errorf("JoinInRoot = %q, want %q", got, want)
	}
}

func TestJoinInRootKeepsEverythingInside(t *testing.T) {
	root := "/mass/"
	for _, name := range []string{"../../escaped", "/etc", "..", "a/../../b"} {
		got, err := JoinInRoot(root, name, "2022.W", "files")
		if err != nil {
			t.Fatalf("JoinInRoot(%q) failed: %v", name, err)
		}
		if !Contains(root, got) {
			t.Errorf("JoinInRoot(%q) = %q, which is outside %q", name, got, root)
		}
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		root, path string
		want       bool
	}{
		{"/mass", "/mass/a/b", true},
		{"/mass", "/mass", true},
		{"/mass", "/mass/../other", false},
		{"/mass", "/massive", false},
		{"/mass", "/etc/passwd", false},
	}
	for _, c := range cases {
		if got := Contains(c.root, c.path); got != c.want {
			t.Errorf("Contains(%q, %q) = %v, want %v", c.root, c.path, got, c.want)
		}
	}
}
