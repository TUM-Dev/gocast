package pathprovider

import (
	"fmt"
	uuid "github.com/iris-contrib/go.uuid"
	"os"
	"strings"
	"testing"
)

/*
Note: these Tests only apply to a Unix-based OS
*/

func TestConfigureWorkerPaths(t *testing.T) {
	//test if hardcoded paths are used when env variables are not set
	var tempDir, storageDir, logDir, persistDir string
	var (
		expectedTempDir    = "/recordings"
		expectedStorageDir = "/mass"
		expectedLogDir     = "/var/log/stream"
		expectedPersistDir = "."
	)

	ConfigureWorkerPaths(&tempDir, &storageDir, &logDir, &persistDir)

	if tempDir != expectedTempDir || storageDir != expectedStorageDir || logDir != expectedLogDir || persistDir != expectedPersistDir {
		t.Error("hardcoded paths incorrect")
		t.FailNow()
	}

	// test if env variables are used when read
	t.Setenv("MassStorage", "/mass/storage")
	t.Setenv("LogDir", "/etc/logs")

	expectedStorageDir = "/mass/storage"
	expectedLogDir = "/etc/logs"

	ConfigureWorkerPaths(&tempDir, &storageDir, &logDir, &persistDir)

	if tempDir != expectedTempDir || storageDir != expectedStorageDir || logDir != expectedLogDir || persistDir != expectedPersistDir {
		t.Error("env paths incorrect")
		t.FailNow()
	}
}

func TestCertDetails(t *testing.T) {
	// test if CertDetails returns correct error when CERT_DIR not set
	var fullChainName, privateKeyName string
	err := CertDetails(&fullChainName, &privateKeyName)

	if !strings.Contains(err.Error(), "could not read cert directory") {
		t.Error("incorrect error message")
		t.FailNow()
	}

	// test if CertDetails writes correct values with no error
	dir := t.TempDir()
	t.Setenv("CERT_DIR", dir)
	f1, err := os.CreateTemp(dir, "*privkey.pem")
	if err != nil {
		t.Error("test could not be executed")
		t.FailNow()
	}

	f2, err := os.CreateTemp(dir, "*fullchain.pem")
	if err != nil {
		t.Error("test could not be executed")
		t.FailNow()
	}

	_, err = os.CreateTemp(dir, "*wrong.pem")
	if err != nil {
		t.Error("test could not be executed")
		t.FailNow()
	}

	err = CertDetails(&fullChainName, &privateKeyName)
	if err != nil {
		t.Error("unexpected error: ", err)
		t.FailNow()
	}
	if f2.Name() != fullChainName || f1.Name() != privateKeyName {
		t.Error("file names incorrect")
		t.FailNow()
	}

	defer os.Remove(f1.Name())
	defer os.Remove(f2.Name())
}

func TestWaveformTemp(t *testing.T) {
	uuid, err := uuid.NewV4()
	if err != nil {
		t.Error("test could not be executed")
		t.FailNow()
	}

	var expectedFilePath = fmt.Sprint("/tmp/", uuid.String(), ".png")

	filename := WaveformTemp(uuid.String())
	if filename != expectedFilePath {
		t.Error("file name incorrect")
		t.FailNow()
	}

}

func TestFfmpegLog(t *testing.T) {
	dir := t.TempDir()

	path := FfmpegLog(dir, "stream")
	expectedPath := fmt.Sprint(dir, "/ffmpeg_stream.log")

	if expectedPath != path {
		t.Error("file name incorrect")
		t.FailNow()
	}
}
