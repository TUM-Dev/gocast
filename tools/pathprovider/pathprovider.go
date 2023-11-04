// Package pathprovider provides file paths in an OS-agnostic manner.
package pathprovider

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Root will return the root directory path for linux or windows
func Root() string {
	return os.Getenv("SystemDrive") + string(os.PathSeparator)
}

var (
	TUMLiveTemporary = filepath.Join(os.TempDir(), "TUM-Live")
	// worker configuration paths
	defaultWorkerTempDir    = filepath.Join(Root(), "recordings")
	defaultWorkerStorageDir = filepath.Join(Root(), "mass")
	defaultWorkerLogDir     = filepath.Join(Root(), "var", "log", "stream")
	defaultWorkerPersistDir = "."
	PersistFileName         = filepath.Join(Root(), "persist.gob")

	// edge paths
	defaultEdgeVodPath = filepath.Join(Root(), "vod")
	EdgeCacheDir       = filepath.Join(os.TempDir(), "edge")
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

// CachedFile returns path to cached file
func CachedFile(filename string) string {
	return filepath.Join(EdgeCacheDir, filename)
}

// VodPath returns path to vod directory,
// first tries env variables then resorts to hardcoded defaults
func VodPath() string {
	vodPath := os.Getenv("VOD_DIR")
	if vodPath == "" {
		vodPath = defaultEdgeVodPath
	}

	return vodPath
}

// ConfigureWorkerPaths writes the TempDir, StorageDir, LogDir and PersistDir paths into the passed pointers,
// first tries env variables then resorts to hardcoded defaults
func ConfigureWorkerPaths(tempDir *string, storageDir *string, logDir *string, persistDir *string) {
	//recordings will end up here before they are converted
	*tempDir = defaultWorkerTempDir

	// recordings will end up here after they are converted
	*storageDir = os.Getenv("MassStorage")
	if *storageDir == "" {
		*storageDir = defaultWorkerStorageDir
	}

	//logging
	*logDir = os.Getenv("LogDir")
	if *logDir == "" {
		*logDir = defaultWorkerLogDir
	}
	*persistDir = os.Getenv("PersistDir")
	if *persistDir == "" {
		*persistDir = defaultWorkerPersistDir
	}

}

// CertDetails returns writes path to certificate and full chain name into the passed pointers,
// returns error for logging purposes
func CertDetails(fullChainName *string, privateKeyName *string) error {
	dirPath := os.Getenv("CERT_DIR")
	dir, err := os.ReadDir(dirPath)

	if err != nil {
		return errors.New(fmt.Sprint("[HTTPS] Skipping, could not read cert directory: ", err))
	}

	for _, entry := range dir {
		if strings.HasSuffix(entry.Name(), "privkey.pem") {
			*privateKeyName = filepath.Join(dirPath, entry.Name())
		}
		if strings.HasSuffix(entry.Name(), "fullchain.pem") {
			*fullChainName = filepath.Join(dirPath, entry.Name())
		}
	}

	if *privateKeyName == "" || *fullChainName == "" {
		return errors.New("[HTTPS] Skipping, could not find privkey.pem or fullchain.pem in cert directory")
	}

	return nil
}
