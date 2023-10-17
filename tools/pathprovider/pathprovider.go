package pathprovider

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	TUMLiveTemporary = filepath.Join(os.TempDir(), "TUM-Live")
	// worker configuration default paths
	defaultWorkerTempDir    = "/recordings"
	defaultWorkerStorageDir = "/mass"
	defaultWorkerLogDir     = "/var/log/stream"
	defaultWorkerPersistDir = "."
)

// LiveThumbnail creates path to thumbnail from streamID
func LiveThumbnail(streamID string) string {
	return filepath.Join(TUMLiveTemporary, fmt.Sprintf("%s.jpeg", streamID))
}

// ConfigureWorkerPaths returns the TempDir, StorageDir, LogDir and PersistDir paths in that order,
// first tries env variables, then resorts to defaults
func ConfigureWorkerPaths() (string, string, string, string) {
	//recordings will end up here before they are converted
	tempDir := defaultWorkerTempDir

	// recordings will end up here after they are converted
	storageDir := os.Getenv("MassStorage")
	if storageDir == "" {
		storageDir = defaultWorkerStorageDir
	}

	//logging
	logDir := os.Getenv("LogDir")
	if logDir == "" {
		logDir = defaultWorkerLogDir
	}
	persistDir := os.Getenv("PersistDir")
	if persistDir == "" {
		persistDir = defaultWorkerPersistDir
	}

	return tempDir, storageDir, logDir, persistDir
}
