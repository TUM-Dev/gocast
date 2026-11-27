package pathprovider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTUMLiveTemporary(t *testing.T) {
	if want := filepath.Join(os.TempDir(), "TUM-Live"); TUMLiveTemporary != want {
		t.Errorf("TUMLiveTemporary = %q, want %q", TUMLiveTemporary, want)
	}
}

// TestLiveThumbnail checks that the path follows TUMLiveTemporary when it is reassigned, as
// livepreview's test does.
func TestLiveThumbnail(t *testing.T) {
	old := TUMLiveTemporary
	defer func() { TUMLiveTemporary = old }()

	if got, want := LiveThumbnail("42"), filepath.Join(old, "42.jpeg"); got != want {
		t.Errorf("LiveThumbnail(42) = %q, want %q", got, want)
	}

	TUMLiveTemporary = t.TempDir()
	if got, want := LiveThumbnail("7"), filepath.Join(TUMLiveTemporary, "7.jpeg"); got != want {
		t.Errorf("LiveThumbnail(7) = %q, want %q", got, want)
	}
}
