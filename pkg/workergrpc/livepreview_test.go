package workergrpc

import (
	"os"
	"strconv"
	"testing"

	"github.com/TUM-Dev/gocast/tools/pathprovider"
)

func TestSaveLivePreview(t *testing.T) {
	old := pathprovider.TUMLiveTemporary
	pathprovider.TUMLiveTemporary = t.TempDir()
	defer func() { pathprovider.TUMLiveTemporary = old }()

	if err := saveLivePreview(42, []byte("jpeg")); err != nil {
		t.Fatalf("saveLivePreview: %v", err)
	}
	got, err := os.ReadFile(pathprovider.LiveThumbnail(strconv.Itoa(42)))
	if err != nil || string(got) != "jpeg" {
		t.Errorf("read back: %q, %v", got, err)
	}
}
