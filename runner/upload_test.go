package runner

import (
	"bytes"
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func TestUploadSlots(t *testing.T) {
	now := time.Now()
	u := newUploadSlots()
	u.now = func() time.Time { return now }

	token, err := u.create(5, "STREAM_VERSION_COMB")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := u.create(5, "STREAM_VERSION_COMB")
	if token == other {
		t.Fatal("tokens must be unique")
	}

	s, ok := u.take(token)
	if !ok || s.streamID != 5 || s.version != "STREAM_VERSION_COMB" {
		t.Errorf("take = %+v, %v", s, ok)
	}
	if _, ok := u.take(token); ok {
		t.Error("a token must only be redeemable once")
	}

	now = now.Add(uploadTokenTTL + time.Second)
	if _, ok := u.take(other); ok {
		t.Error("an expired token was accepted")
	}
	if _, ok := u.take("unknown"); ok {
		t.Error("an unknown token was accepted")
	}

	// creating prunes what expired
	_, _ = u.create(1, "x")
	if len(u.slots) != 1 {
		t.Errorf("%d slots after pruning, want 1", len(u.slots))
	}
}

func newUploadTestRunner(t *testing.T) *Runner {
	t.Helper()
	config.Config.UploadPath = t.TempDir()
	r := newTestRunner()
	r.uploads = newUploadSlots()
	return r
}

func multipartBody(t *testing.T, field, content string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if field != "" {
		fw, err := w.CreateFormFile(field, "../../etc/lecture.mp4")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write([]byte(content))
	}
	_ = w.Close()
	return body, w.FormDataContentType()
}

func post(r *Runner, token string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/upload/"+token, body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	r.uploadHandler().ServeHTTP(rec, req)
	return rec
}

func TestHandleUpload(t *testing.T) {
	t.Run("stores the file and starts the import", func(t *testing.T) {
		r := newUploadTestRunner(t)
		token, _ := r.uploads.create(9, "STREAM_VERSION_CAMERA")
		body, ct := multipartBody(t, "file", "video-bytes")

		// The import itself is covered in pkg/actions; here only the hand-over matters.
		var got map[string]any
		started := make(chan struct{})
		prev := importActions
		importActions = []actions.Action{func(_ context.Context, _ *slog.Logger, _ chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
			got = d
			close(started)
			return nil
		}}
		t.Cleanup(func() { importActions = prev })

		if rec := post(r, token, body, ct); rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", rec.Code, rec.Body)
		}
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("import not started")
		}
		if got["streamID"] != uint64(9) || got["streamVersion"] != "STREAM_VERSION_CAMERA" {
			t.Errorf("job data = %v", got)
		}
		b, err := os.ReadFile(got["uploadPath"].(string))
		if err != nil || string(b) != "video-bytes" {
			t.Errorf("stored file = %q, %v", b, err)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		r := newUploadTestRunner(t)
		body, ct := multipartBody(t, "file", "x")
		if rec := post(r, "nope", body, ct); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("token is single use", func(t *testing.T) {
		r := newUploadTestRunner(t)
		token, _ := r.uploads.create(9, "STREAM_VERSION_CAMERA")
		body, ct := multipartBody(t, "other", "x")
		post(r, token, body, ct)
		body, ct = multipartBody(t, "file", "x")
		if rec := post(r, token, body, ct); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("no file", func(t *testing.T) {
		r := newUploadTestRunner(t)
		token, _ := r.uploads.create(9, "STREAM_VERSION_CAMERA")
		body, ct := multipartBody(t, "other", "x")
		if rec := post(r, token, body, ct); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if entries, _ := os.ReadDir(config.Config.UploadPath); len(entries) != 0 {
			t.Errorf("%d files left in the upload dir", len(entries))
		}
	})

	t.Run("empty file", func(t *testing.T) {
		r := newUploadTestRunner(t)
		token, _ := r.uploads.create(9, "STREAM_VERSION_CAMERA")
		body, ct := multipartBody(t, "file", "")
		if rec := post(r, token, body, ct); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if entries, _ := os.ReadDir(config.Config.UploadPath); len(entries) != 0 {
			t.Errorf("%d files left in the upload dir", len(entries))
		}
	})

	t.Run("not multipart", func(t *testing.T) {
		r := newUploadTestRunner(t)
		token, _ := r.uploads.create(9, "STREAM_VERSION_CAMERA")
		if rec := post(r, token, bytes.NewBufferString("raw"), "video/mp4"); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

func TestPrepareUpload(t *testing.T) {
	r := newUploadTestRunner(t)
	config.Config.UploadPort = 8188

	for name, req := range map[string]*protobuf.PrepareUploadRequest{
		"no stream":       {Version: protobuf.StreamVersion_STREAM_VERSION_COMBINED.Enum()},
		"no version":      {StreamId: uptr(1)},
		"unspecified":     {StreamId: uptr(1), Version: protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED.Enum()},
		"unknown version": {StreamId: uptr(1), Version: ver(99)},
	} {
		_, err := r.PrepareUpload(context.Background(), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: code = %v, want InvalidArgument", name, status.Code(err))
		}
	}

	resp, err := r.PrepareUpload(context.Background(), &protobuf.PrepareUploadRequest{
		StreamId: uptr(4), Version: protobuf.StreamVersion_STREAM_VERSION_PRESENTATION.Enum(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetUploadPort() != 8188 {
		t.Errorf("port = %d, want 8188", resp.GetUploadPort())
	}
	s, ok := r.uploads.take(resp.GetToken())
	if !ok || s.streamID != 4 || s.version != "STREAM_VERSION_PRESENTATION" {
		t.Errorf("token resolves to %+v, %v", s, ok)
	}
}

func uptr(v uint64) *uint64 { return &v }
func ver(v int32) *protobuf.StreamVersion {
	s := protobuf.StreamVersion(v)
	return &s
}
