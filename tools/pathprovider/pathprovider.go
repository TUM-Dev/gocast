package pathprovider

import (
	"fmt"
	"os"
	"path/filepath"
)

// TUMLiveTemporary is the path at which temporary files like in-progress thumbnails are stored.
var TUMLiveTemporary = filepath.Join(os.TempDir(), "TUM-Live")

// LiveThumbnail returns the path to the thumbnail of a livestream.
func LiveThumbnail(streamID string) string {
	return filepath.Join(TUMLiveTemporary, fmt.Sprintf("%s.jpeg", streamID))
}
