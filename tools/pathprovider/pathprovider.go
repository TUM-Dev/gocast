package pathprovider

import (
	"fmt"
	"os"
	"path/filepath"
)

const PersistFileName = "/persist.gob"

// root will return the root directory path for linux or windows
func root() string {
	return os.Getenv("SystemDrive") + string(os.PathSeparator)
}

var (
	TUMLiveTemporary = filepath.Join(os.TempDir(), "TUM-Live")
	// worker configuration default paths
	defaultWorkerTempDir    = filepath.Join(root(), "recordings")
	defaultWorkerStorageDir = filepath.Join(root(), "mass")
	defaultWorkerLogDir     = filepath.Join(root(), "var", "log", "stream")
	defaultWorkerPersistDir = "."
)

// LiveThumbnail creates path to thumbnail from streamID
func LiveThumbnail(streamID string) string {
	return filepath.Join(TUMLiveTemporary, fmt.Sprintf("%s.jpeg", streamID))
}

// FfmpegLog creates path to log file from stream name
func FfmpegLog(logDir string, streamName string) string {
	return filepath.Join(logDir, fmt.Sprintf("ffmpeg_%s.log", streamName))
}

// WaveformTemp creates path to temp file for waveforms from uuid
func WaveformTemp(uuid string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s.png", uuid))
}

// ConfigureWorkerPaths returns the TempDir, StorageDir, LogDir and PersistDir paths in that order,
// first tries env variables then resorts to defaults
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
