package pathprovider

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	TUMLiveTemporary = filepath.Join(os.TempDir(), "TUM-Live")
)

func LiveThumbnail(streamID string) string {
	return filepath.Join(TUMLiveTemporary, fmt.Sprintf("%s.jpeg", streamID))
}
