package pathprovider

import (
	"os"
	"path/filepath"
)

var (
	TUMLiveTemporary = filepath.Join(os.TempDir(), "TUM-Live")
)
