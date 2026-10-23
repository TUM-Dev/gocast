package actions

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func TestMkOfflinePackage(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	storage := t.TempDir()
	config.Config.StoragePath = storage

	// A VOD as MkVOD and ImportVOD leave it: storage/<stream>/<version>/playlist.m3u8.
	vod := filepath.Join(storage, "7", "STREAM_VERSION_COMBINED")
	if err := os.MkdirAll(vod, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	upload := makeUpload(t, "src.mp4", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
	if out, err := exec.Command("ffmpeg", importArgs(upload, vod, false)...).CombinedOutput(); err != nil {
		t.Fatalf("making the source VOD: %v (%s)", err, out)
	}

	key := []byte("0123456789abcdef")
	notify := make(chan *protobuf.Notification, 4)
	data := map[string]any{
		"packageID": uint64(42), "streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED",
		"key": key, "keyURI": "gocast-offline:42",
	}
	if err := MkOfflinePackage(context.Background(), testLogger(), notify, data, nil); err != nil {
		t.Fatalf("MkOfflinePackage = %v", err)
	}

	dir := filepath.Join(storage, "offline", "42")
	playlist, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("no playlist: %v", err)
	}
	if !strings.Contains(string(playlist), `#EXT-X-KEY:METHOD=AES-128,URI="gocast-offline:42"`) {
		t.Errorf("playlist is not encrypted with the given key URI:\n%s", playlist)
	}

	// HLS AES-128 without an IV line: AES-CBC, IV = media sequence number, PKCS#7.
	segment, err := os.ReadFile(filepath.Join(dir, "00000.ts"))
	if err != nil {
		t.Fatalf("no first segment: %v", err)
	}
	if segment[0] == 0x47 {
		t.Error("first segment starts with the TS sync byte: it is not encrypted")
	}
	block, _ := aes.NewCipher(key)
	iv := make([]byte, 16)
	binary.BigEndian.PutUint64(iv[8:], 0)
	plain := make([]byte, len(segment))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, segment)
	if plain[0] != 0x47 {
		t.Errorf("decrypted segment starts with %#x, want the TS sync byte", plain[0])
	}

	ready := (<-notify).GetOfflinePackageReady()
	if ready.GetPackageId() != 42 || ready.GetSegmentCount() == 0 || ready.GetSizeBytes() == 0 {
		t.Errorf("ready notification = %v", ready)
	}

	// Nothing of the key may stay behind next to the package.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		if !strings.HasSuffix(e.Name(), ".ts") && bytes.Contains(b, key) {
			t.Errorf("%s contains the key", e.Name())
		}
	}
	assertNoLeftovers(t, filepath.Join(storage, "offline"), "42")
}

func TestMkOfflinePackageWithoutVOD(t *testing.T) {
	config.Config.StoragePath = t.TempDir()
	notify := make(chan *protobuf.Notification, 1)
	data := map[string]any{
		"packageID": uint64(1), "streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED",
		"key": []byte("0123456789abcdef"),
	}
	err := MkOfflinePackage(context.Background(), testLogger(), notify, data, nil)
	if err == nil || !IsAbortingError(err) {
		t.Fatalf("MkOfflinePackage = %v, want an aborting error", err)
	}
	if got := (<-notify).GetOfflinePackageFailed().GetReason(); got != reasonOfflineNoVOD {
		t.Errorf("reason = %q, want %q", got, reasonOfflineNoVOD)
	}
}
