package apiv2

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// fakeRecordings keeps browser recordings in memory, as the database would.
type fakeRecordings struct {
	mu   sync.Mutex
	next uint
	recs map[uint]model.BrowserRecording
	// onGet, when set, sees every GetBrowserRecording, as another request between two
	// reads would.
	onGet func(id uint)
	// failHandedOff fails that many calls of MarkBrowserRecordingHandedOff.
	failHandedOff int
}

var _ dao.BrowserRecordingDao = (*fakeRecordings)(nil)

func newFakeRecordings() *fakeRecordings {
	return &fakeRecordings{recs: map[uint]model.BrowserRecording{}}
}

func (f *fakeRecordings) CreateBrowserRecording(_ context.Context, rec *model.BrowserRecording) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	rec.ID, rec.CreatedAt = f.next, time.Now()
	f.recs[rec.ID] = *rec
	return nil
}

func (f *fakeRecordings) GetBrowserRecording(_ context.Context, id uint) (model.BrowserRecording, error) {
	if f.onGet != nil {
		f.onGet(id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.recs[id]
	if !ok {
		return rec, gorm.ErrRecordNotFound
	}
	return rec, nil
}

func (f *fakeRecordings) GetBrowserRecordingByTicket(_ context.Context, hash []byte) (model.BrowserRecording, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, rec := range f.recs {
		if bytes.Equal(rec.TicketHash, hash) {
			return rec, nil
		}
	}
	return model.BrowserRecording{}, gorm.ErrRecordNotFound
}

func (f *fakeRecordings) filter(keep func(model.BrowserRecording) bool) []model.BrowserRecording {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.BrowserRecording
	for _, rec := range f.recs {
		if keep(rec) {
			out = append(out, rec)
		}
	}
	slices.SortFunc(out, func(a, b model.BrowserRecording) int { return int(a.ID) - int(b.ID) })
	return out
}

func (f *fakeRecordings) ListBrowserRecordings(_ context.Context, streamID uint) ([]model.BrowserRecording, error) {
	return f.filter(func(r model.BrowserRecording) bool { return r.StreamID == streamID && r.ConvertedAt == nil }), nil
}

func (f *fakeRecordings) update(id uint, change func(*model.BrowserRecording) bool) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.recs[id]
	if !ok || !change(&rec) {
		return false
	}
	f.recs[id] = rec
	return true
}

func (f *fakeRecordings) FinalizeBrowserRecording(_ context.Context, id uint, now time.Time) (bool, error) {
	return f.update(id, func(r *model.BrowserRecording) bool {
		if r.FinalizedAt != nil {
			return false
		}
		r.FinalizedAt = &now
		return true
	}), nil
}

func (f *fakeRecordings) ListPendingHandOffs(context.Context) ([]model.BrowserRecording, error) {
	return f.filter(func(r model.BrowserRecording) bool {
		return r.FinalizedAt != nil && r.HandedOffAt == nil && r.HandOffAttempts < model.MaxBrowserRecordingHandOffAttempts
	}), nil
}

func (f *fakeRecordings) ListUnfinalizedBrowserRecordings(_ context.Context, before time.Time) ([]model.BrowserRecording, error) {
	return f.filter(func(r model.BrowserRecording) bool { return r.FinalizedAt == nil && r.CreatedAt.Before(before) }), nil
}

func (f *fakeRecordings) MarkBrowserRecordingHandedOff(ctx context.Context, id uint, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	fail := f.failHandedOff > 0
	if fail {
		f.failHandedOff--
	}
	f.mu.Unlock()
	if fail {
		return errors.New("database blinked")
	}
	f.update(id, func(r *model.BrowserRecording) bool { r.HandedOffAt, r.HandOffError = &now, ""; return true })
	return nil
}

func (f *fakeRecordings) MarkBrowserRecordingHandOffFailed(ctx context.Context, id uint, reason string, counts bool) error {
	// As the database does: an expired context writes nothing.
	if err := ctx.Err(); err != nil {
		return err
	}
	f.update(id, func(r *model.BrowserRecording) bool {
		r.HandOffError = reason
		if counts {
			r.HandOffAttempts++
		}
		return true
	})
	return nil
}

func (f *fakeRecordings) ResetBrowserRecordingHandOff(_ context.Context, id uint) error {
	f.update(id, func(r *model.BrowserRecording) bool { r.HandOffAttempts = 0; return true })
	return nil
}

func (f *fakeRecordings) ListConvertingBrowserRecordings(context.Context) ([]model.BrowserRecording, error) {
	return f.filter(func(r model.BrowserRecording) bool { return r.HandedOffAt != nil && r.ConvertedAt == nil }), nil
}

func (f *fakeRecordings) MarkBrowserRecordingConverted(_ context.Context, id uint, now time.Time) error {
	f.update(id, func(r *model.BrowserRecording) bool { r.ConvertedAt = &now; return true })
	return nil
}

func (f *fakeRecordings) RequeueBrowserRecordingHandOff(_ context.Context, id uint, reason string) error {
	f.update(id, func(r *model.BrowserRecording) bool {
		r.HandedOffAt, r.HandOffError = nil, reason
		r.HandOffAttempts++
		return true
	})
	return nil
}

func (f *fakeRecordings) DeleteBrowserRecording(_ context.Context, id uint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.recs, id)
	return nil
}

// statusOf is the HTTP status the gateway answers err with, 200 for none.
func statusOf(err error) int {
	if err == nil {
		return http.StatusOK
	}
	return runtime.HTTPStatusFromCode(status.Convert(err).Code())
}

type recordingEnv struct {
	api  *API
	recs *fakeRecordings
	mux  *http.ServeMux
	// The lecture and its progress rows, which the runner's reports would change.
	mu       *sync.Mutex
	lecture  *model.Stream
	progress map[model.StreamVersion]model.TranscodingProgress
}

// converted is what the runner's VODReady leaves: the playlist, and no progress row.
func (env recordingEnv) converted(v model.StreamVersion) {
	env.mu.Lock()
	defer env.mu.Unlock()
	delete(env.progress, v)
	*playlistOf(env.lecture, v) = "https://edge/" + string(v) + ".m3u8"
}

// failed is what the runner's VODFailed leaves.
func (env recordingEnv) failed(v model.StreamVersion, reason string) {
	env.mu.Lock()
	defer env.mu.Unlock()
	env.progress[v] = model.TranscodingProgress{Version: v, Failed: true, Error: reason}
}

func newRecordingAPI(t *testing.T, uploads Uploads, lecture model.Stream) recordingEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	streams := mock_dao.NewMockStreamsDao(ctrl)
	env := recordingEnv{mu: &sync.Mutex{}, lecture: &lecture, progress: map[model.StreamVersion]model.TranscodingProgress{}}
	streams.EXPECT().GetStreamByID(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, string) (model.Stream, error) {
		env.mu.Lock()
		defer env.mu.Unlock()
		return *env.lecture, nil
	}).AnyTimes()
	streams.EXPECT().GetTranscodingProgressByVersion(gomock.Any(), gomock.Any()).DoAndReturn(func(v model.StreamVersion, _ uint) (model.TranscodingProgress, error) {
		env.mu.Lock()
		defer env.mu.Unlock()
		p, ok := env.progress[v]
		if !ok {
			return p, gorm.ErrRecordNotFound
		}
		return p, nil
	}).AnyTimes()
	streams.EXPECT().SaveTranscodingProgress(gomock.Any()).DoAndReturn(func(p model.TranscodingProgress) error {
		env.mu.Lock()
		defer env.mu.Unlock()
		env.progress[p.Version] = p
		return nil
	}).AnyTimes()
	streams.EXPECT().RemoveTranscodingProgress(gomock.Any(), gomock.Any()).DoAndReturn(func(v model.StreamVersion, _ uint) error {
		env.mu.Lock()
		defer env.mu.Unlock()
		delete(env.progress, v)
		return nil
	}).AnyTimes()
	recs := newFakeRecordings()
	api := &API{
		dao:         dao.DaoWrapper{StreamsDao: streams, BrowserRecordingDao: recs, AuditDao: auditSink(t)},
		log:         slog.Default(),
		uploads:     uploads,
		massStorage: t.TempDir(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(browserRecordingChunkPath, api.handleBrowserRecordingChunk)
	env.api, env.recs, env.mux = api, recs, mux
	return env
}

func (env recordingEnv) create(t *testing.T, version string) *protobuf.CreateBrowserRecordingResponse {
	t.Helper()
	resp, err := env.api.CreateBrowserRecording(asCaller(lectureAdmin), &protobuf.CreateBrowserRecordingRequest{
		CourseId: 1, StreamId: 7, Version: version, MimeType: "video/webm",
	})
	if err != nil {
		t.Fatalf("CreateBrowserRecording: %v", err)
	}
	return resp
}

func (env recordingEnv) put(uploadPath, segment, seq, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, uploadPath+"/segments/"+segment+"/chunks/"+seq, strings.NewReader(body))
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	return rec
}

func (env recordingEnv) mustPut(t *testing.T, uploadPath, segment, seq, body string) {
	t.Helper()
	if rec := env.put(uploadPath, segment, seq, body); rec.Code != http.StatusOK {
		t.Fatalf("PUT %s/%s: %d %s", segment, seq, rec.Code, rec.Body)
	}
}

func (env recordingEnv) finalize(id uint32, expected ...uint32) (*protobuf.BrowserRecording, error) {
	resp, err := env.api.FinalizeBrowserRecording(asCaller(lectureAdmin), &protobuf.FinalizeBrowserRecordingRequest{
		CourseId: 1, StreamId: 7, RecordingId: id, Expected: expected,
	})
	env.api.handOffs.Wait()
	return resp, err
}

// recordingRunner is a runner's upload endpoint that keeps the file parts it was sent.
func recordingRunner(t *testing.T, status int) (*fakeUploads, *[]string) {
	t.Helper()
	var parts []string
	uploads := &fakeUploads{}
	uploads.target = fakeRunner(t, func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Errorf("runner: %v", err)
			return
		}
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			b, _ := io.ReadAll(p)
			parts = append(parts, p.FormName()+"="+string(b))
		}
		if status != http.StatusOK {
			http.Error(w, "refused", status)
		}
	})
	return uploads, &parts
}

func TestBrowserRecording(t *testing.T) {
	t.Run("chunks arrive, finalize waits for the missing ones, the runner gets each segment joined", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "PRES")
		path := created.GetUploadPath()
		if !strings.HasPrefix(path, browserRecordingPrefix) {
			t.Fatalf("upload path = %q", path)
		}

		env.mustPut(t, path, "0", "0", "a")
		env.mustPut(t, path, "0", "2", "c") // ahead of a gap
		env.mustPut(t, path, "1", "0", "x")

		got, err := env.api.GetBrowserRecording(asCaller(lectureAdmin), &protobuf.GetBrowserRecordingRequest{
			CourseId: 1, StreamId: 7, RecordingId: created.GetRecording().GetId(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.GetReceived(), []uint32{1, 1}) {
			t.Errorf("received = %v, want [1 1]", got.GetReceived())
		}

		if _, err := env.finalize(created.GetRecording().GetId(), 3, 1); statusOf(err) != http.StatusConflict {
			t.Fatalf("finalize with chunks missing: %v, want 409", err)
		}
		if uploads.called {
			t.Fatal("a runner was reserved for an incomplete recording")
		}

		env.mustPut(t, path, "0", "1", "b")
		if _, err := env.finalize(created.GetRecording().GetId(), 3, 1); err != nil {
			t.Fatalf("finalize: %v", err)
		}
		if want := []string{"file=abc", "file=x"}; !slices.Equal(*parts, want) {
			t.Errorf("runner got %v, want %v", *parts, want)
		}
		if uploads.streamID != 7 || uploads.version != model.PRES {
			t.Errorf("reserved %d %s", uploads.streamID, uploads.version)
		}
		rec, _ := env.recs.GetBrowserRecording(context.Background(), uint(created.GetRecording().GetId()))
		if rec.HandedOffAt == nil {
			t.Error("not marked as handed off")
		}
		if rec := env.put(path, "0", "3", "d"); rec.Code != http.StatusConflict {
			t.Errorf("chunk after finalize: %d, want 409", rec.Code)
		}

		// The runner has the file, but could still lose it: the chunks stay.
		store, _ := env.api.recordingStore()
		env.api.CheckBrowserRecordingConversions()
		if got, _ := store.received(rec.ID); !slices.Equal(got, []uint32{3, 1}) {
			t.Errorf("received while converting = %v, want the chunks kept", got)
		}
		env.converted(model.PRES)
		env.api.CheckBrowserRecordingConversions()
		if rec, _ := env.recs.GetBrowserRecording(context.Background(), rec.ID); rec.ConvertedAt == nil {
			t.Error("not marked as converted once the VOD is there")
		}
		if _, err := os.Stat(store.recordingDir(rec.ID)); !os.IsNotExist(err) {
			t.Errorf("chunks left after the conversion: %v", err)
		}
		listed, _ := env.api.ListBrowserRecordings(asCaller(lectureAdmin), &protobuf.ListBrowserRecordingsRequest{CourseId: 1, StreamId: 7})
		if len(listed.GetRecordings()) != 0 {
			t.Errorf("a converted recording is still listed: %v", listed.GetRecordings())
		}
	})

	t.Run("a failed hand-off keeps the chunks and counts", func(t *testing.T) {
		uploads, _ := recordingRunner(t, http.StatusInternalServerError)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "CAM")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.finalize(created.GetRecording().GetId(), 1); err != nil {
			t.Fatal(err)
		}
		rec, _ := env.recs.GetBrowserRecording(context.Background(), uint(created.GetRecording().GetId()))
		if rec.HandedOffAt != nil || rec.HandOffAttempts != 1 || rec.HandOffError == "" {
			t.Errorf("after a refusal: %+v", rec)
		}
		store, _ := env.api.recordingStore()
		if got, _ := store.received(rec.ID); !slices.Equal(got, []uint32{1}) {
			t.Errorf("received = %v, want the chunk kept", got)
		}
	})

	t.Run("no runner doesn't count towards the limit", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{err: errNoRunner}, courseOneLecture())
		created := env.create(t, "CAM")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatal(err)
		}
		rec, _ := env.recs.GetBrowserRecording(context.Background(), uint(created.GetRecording().GetId()))
		if rec.HandOffAttempts != 0 || rec.HandOffError == "" {
			t.Errorf("after no runner: %+v", rec)
		}
	})

	t.Run("finalizing again after giving up retries", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "CAM")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		id := uint(created.GetRecording().GetId())
		now := time.Now()
		env.recs.update(id, func(r *model.BrowserRecording) bool {
			r.FinalizedAt, r.HandOffAttempts = &now, model.MaxBrowserRecordingHandOffAttempts
			return true
		})
		env.api.ResumeBrowserRecordingHandOffs()
		env.api.handOffs.Wait()
		if len(*parts) != 0 {
			t.Fatal("handed off on its own after giving up")
		}
		if _, err := env.finalize(uint32(id)); err != nil {
			t.Fatal(err)
		}
		if len(*parts) != 1 {
			t.Errorf("runner got %v", *parts)
		}
	})

	t.Run("nothing arrived is refused", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
		created := env.create(t, "PRES")
		if _, err := env.finalize(created.GetRecording().GetId()); statusOf(err) != http.StatusBadRequest {
			t.Errorf("finalize: %v, want 400", err)
		}
	})

	t.Run("resumes a hand-off a restart cut off", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "PRES")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		env.recs.FinalizeBrowserRecording(context.Background(), uint(created.GetRecording().GetId()), time.Now())

		env.api.ResumeBrowserRecordingHandOffs()
		env.api.handOffs.Wait()
		if !slices.Equal(*parts, []string{"file=a"}) {
			t.Errorf("runner got %v", *parts)
		}
	})

	t.Run("an idle recording is finalized with what arrived, an empty one removed", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		withChunks := env.create(t, "PRES")
		empty := env.create(t, "CAM")
		env.mustPut(t, withChunks.GetUploadPath(), "0", "0", "a")

		old := time.Now().Add(-browserRecordingIdleTimeout - time.Hour)
		for _, id := range []uint32{withChunks.GetRecording().GetId(), empty.GetRecording().GetId()} {
			env.recs.update(uint(id), func(r *model.BrowserRecording) bool { r.CreatedAt = old; return true })
		}
		store, _ := env.api.recordingStore()
		_ = filepath.Walk(store.root, func(p string, _ os.FileInfo, _ error) error { return os.Chtimes(p, old, old) })

		env.api.FinalizeIdleBrowserRecordings()
		env.api.handOffs.Wait()
		if !slices.Equal(*parts, []string{"file=a"}) {
			t.Errorf("runner got %v", *parts)
		}
		if _, err := env.recs.GetBrowserRecording(context.Background(), uint(empty.GetRecording().GetId())); err == nil {
			t.Error("the empty recording is still there")
		}
	})

	t.Run("a recording still taking chunks is not idle", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
		created := env.create(t, "PRES")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		id := uint(created.GetRecording().GetId())
		env.recs.update(id, func(r *model.BrowserRecording) bool {
			r.CreatedAt = time.Now().Add(-browserRecordingIdleTimeout - time.Hour)
			return true
		})
		env.api.FinalizeIdleBrowserRecordings()
		if rec, _ := env.recs.GetBrowserRecording(context.Background(), id); rec.FinalizedAt != nil {
			t.Error("finalized a recording that took a chunk just now")
		}
	})

	t.Run("abandon removes what arrived", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
		created := env.create(t, "PRES")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.api.AbandonBrowserRecording(asCaller(lectureAdmin), &protobuf.AbandonBrowserRecordingRequest{
			CourseId: 1, StreamId: 7, RecordingId: created.GetRecording().GetId(),
		}); err != nil {
			t.Fatal(err)
		}
		if rec := env.put(created.GetUploadPath(), "0", "1", "b"); rec.Code != http.StatusNotFound {
			t.Errorf("chunk after abandon: %d, want 404", rec.Code)
		}
		store, _ := env.api.recordingStore()
		if got, _ := store.received(uint(created.GetRecording().GetId())); got != nil {
			t.Errorf("received after abandon = %v", got)
		}
	})

	t.Run("chunks left under a reused ID are not joined into a new recording", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		store, _ := env.api.recordingStore()
		// What a recording 1 left before the database was restored from a backup.
		if err := store.put(1, 0, 1, strings.NewReader("stale")); err != nil {
			t.Fatal(err)
		}
		created := env.create(t, "PRES")
		if created.GetRecording().GetId() != 1 {
			t.Fatalf("got recording %d, want the reused ID 1", created.GetRecording().GetId())
		}
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.finalize(1); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(*parts, []string{"file=a"}) {
			t.Errorf("runner got %v, want only the new chunk", *parts)
		}
	})

	t.Run("create refuses COMB and a live lecture", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
		if _, err := env.api.CreateBrowserRecording(asCaller(lectureAdmin), &protobuf.CreateBrowserRecordingRequest{
			CourseId: 1, StreamId: 7, Version: "COMB",
		}); statusOf(err) != http.StatusBadRequest {
			t.Errorf("COMB: %v, want 400", err)
		}
		live := courseOneLecture()
		live.LiveNow = true
		env = newRecordingAPI(t, &fakeUploads{}, live)
		if _, err := env.api.CreateBrowserRecording(asCaller(lectureAdmin), &protobuf.CreateBrowserRecordingRequest{
			CourseId: 1, StreamId: 7, Version: "PRES",
		}); statusOf(err) != http.StatusConflict {
			t.Errorf("live: %v, want 409", err)
		}
	})

	t.Run("a recording of another lecture is not found", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
		created := env.create(t, "PRES")
		env.recs.update(uint(created.GetRecording().GetId()), func(r *model.BrowserRecording) bool { r.StreamID = 99; return true })
		if _, err := env.api.GetBrowserRecording(asCaller(lectureAdmin), &protobuf.GetBrowserRecordingRequest{
			CourseId: 1, StreamId: 7, RecordingId: created.GetRecording().GetId(),
		}); statusOf(err) != http.StatusNotFound {
			t.Errorf("got %v, want 404", err)
		}
	})
}

func TestBrowserRecordingRaces(t *testing.T) {
	t.Run("a chunk still arriving when the recording is finalized is refused, not lost", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "PRES")
		path := created.GetUploadPath()
		env.mustPut(t, path, "0", "0", "a")

		// The second chunk's request passes the handler's first check, then its body
		// takes a while.
		body, writer := io.Pipe()
		answered := make(chan int)
		go func() {
			req := httptest.NewRequest(http.MethodPut, path+"/segments/0/chunks/1", body)
			rec := httptest.NewRecorder()
			env.mux.ServeHTTP(rec, req)
			answered <- rec.Code
		}()
		_, _ = writer.Write([]byte("b"))

		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatalf("finalize: %v", err)
		}
		_ = writer.Close()
		if code := <-answered; code != http.StatusConflict {
			t.Errorf("late chunk answered %d, want 409 so the browser keeps it", code)
		}
		if !slices.Equal(*parts, []string{"file=a"}) {
			t.Errorf("runner got %v", *parts)
		}
		store, _ := env.api.recordingStore()
		if got, _ := store.received(uint(created.GetRecording().GetId())); !slices.Equal(got, []uint32{1}) {
			t.Errorf("received = %v, want the late chunk left out", got)
		}
		if entries, _ := os.ReadDir(store.stagingDir()); len(entries) != 0 {
			t.Errorf("%d staged chunks left", len(entries))
		}
	})

	t.Run("a hand-off that times out still counts", func(t *testing.T) {
		prev := handOffTimeout
		handOffTimeout = 50 * time.Millisecond
		t.Cleanup(func() { handOffTimeout = prev })
		uploads := &fakeUploads{}
		uploads.target = fakeRunner(t, func(w http.ResponseWriter, r *http.Request) {
			// A runner that takes the file and then never answers. The server only
			// notices the client giving up once the body is read.
			_, _ = io.Copy(io.Discard, r.Body)
			<-r.Context().Done()
		})
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "PRES")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatal(err)
		}
		rec, _ := env.recs.GetBrowserRecording(context.Background(), uint(created.GetRecording().GetId()))
		if rec.HandOffAttempts != 1 || rec.HandOffError == "" {
			t.Errorf("after a timeout: attempts %d, error %q", rec.HandOffAttempts, rec.HandOffError)
		}
	})

	t.Run("a hand-off the database failed to record is not sent again", func(t *testing.T) {
		prev := markRetryDelay
		markRetryDelay = time.Millisecond
		t.Cleanup(func() { markRetryDelay = prev })
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		env.recs.failHandedOff = 3
		created := env.create(t, "PRES")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatal(err)
		}

		env.api.ResumeBrowserRecordingHandOffs()
		env.api.handOffs.Wait()
		if len(*parts) != 1 {
			t.Errorf("runner got the recording %d times: %v", len(*parts), *parts)
		}
		rec, _ := env.recs.GetBrowserRecording(context.Background(), uint(created.GetRecording().GetId()))
		if rec.HandedOffAt == nil {
			t.Error("not taken as handed off once the database was back")
		}
	})

	t.Run("abandon is refused when a hand-off finished after its first look", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
		created := env.create(t, "PRES")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		id := uint(created.GetRecording().GetId())
		gets := 0
		env.recs.onGet = func(uint) {
			// The first read sees it waiting; by the second a hand-off is through.
			if gets++; gets == 2 {
				now := time.Now()
				env.recs.update(id, func(r *model.BrowserRecording) bool { r.FinalizedAt, r.HandedOffAt = &now, &now; return true })
			}
		}
		_, err := env.api.AbandonBrowserRecording(asCaller(lectureAdmin), &protobuf.AbandonBrowserRecordingRequest{
			CourseId: 1, StreamId: 7, RecordingId: created.GetRecording().GetId(),
		})
		if statusOf(err) != http.StatusConflict {
			t.Errorf("abandon: %v, want 409", err)
		}
		if _, err := env.recs.GetBrowserRecording(context.Background(), id); err != nil {
			t.Errorf("the handed-off recording was deleted: %v", err)
		}
	})

	t.Run("a hand-off waits while another video of the version converts", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		env.progress[model.PRES] = model.TranscodingProgress{Version: model.PRES, Progress: 40}
		created := env.create(t, "PRES")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatal(err)
		}
		if uploads.called || len(*parts) != 0 {
			t.Fatal("handed off while another video of the version converts")
		}
		rec, _ := env.recs.GetBrowserRecording(context.Background(), uint(created.GetRecording().GetId()))
		if rec.HandOffAttempts != 0 {
			t.Errorf("waiting counted as a failed attempt: %+v", rec)
		}

		env.converted(model.PRES)
		env.api.ResumeBrowserRecordingHandOffs()
		env.api.handOffs.Wait()
		if len(*parts) != 1 {
			t.Errorf("runner got %v once the version was free", *parts)
		}
	})

	t.Run("abandon is refused while a hand-off runs", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
		created := env.create(t, "PRES")
		env.api.handingOff.Store(uint(created.GetRecording().GetId()), struct{}{})
		_, err := env.api.AbandonBrowserRecording(asCaller(lectureAdmin), &protobuf.AbandonBrowserRecordingRequest{
			CourseId: 1, StreamId: 7, RecordingId: created.GetRecording().GetId(),
		})
		if statusOf(err) != http.StatusConflict {
			t.Errorf("abandon: %v, want 409", err)
		}
	})
}

func TestBrowserRecordingConversion(t *testing.T) {
	t.Run("a failed conversion is handed off again", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "CAM")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatal(err)
		}
		env.failed(model.CAM, "The conversion was interrupted. Please upload the video again.")
		env.api.CheckBrowserRecordingConversions()
		env.api.handOffs.Wait()

		if len(*parts) != 2 {
			t.Errorf("runner got %v, want the recording twice", *parts)
		}
		rec, _ := env.recs.GetBrowserRecording(context.Background(), uint(created.GetRecording().GetId()))
		if rec.HandedOffAt == nil || rec.HandOffAttempts != 1 {
			t.Errorf("after a failed conversion: %+v", rec)
		}
	})

	t.Run("a conversion that never finishes is taken as lost", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "CAM")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatal(err)
		}
		long := time.Now().Add(-lostConversionAfter - time.Minute)
		env.recs.update(uint(created.GetRecording().GetId()), func(r *model.BrowserRecording) bool { r.HandedOffAt = &long; return true })

		env.api.CheckBrowserRecordingConversions()
		env.api.handOffs.Wait()
		if len(*parts) != 2 {
			t.Errorf("runner got %v, want the recording again", *parts)
		}
	})

	t.Run("the runner learns of the gap before a segment", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "PRES")
		path := created.GetUploadPath()
		env.mustPut(t, path, "0", "0", "a")
		if rec := env.put(path+"", "1", "0?gap_ms=1500", "x"); rec.Code != http.StatusOK {
			t.Fatalf("PUT with gap: %d %s", rec.Code, rec.Body)
		}
		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatal(err)
		}
		if want := []string{"file=a", "gap_ms=1500", "file=x"}; !slices.Equal(*parts, want) {
			t.Errorf("runner got %v, want %v", *parts, want)
		}
	})

	t.Run("the time a version missed at the end goes after its last file", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "PRES")
		env.mustPut(t, created.GetUploadPath(), "0", "0", "a")
		if _, err := env.api.FinalizeBrowserRecording(asCaller(lectureAdmin), &protobuf.FinalizeBrowserRecordingRequest{
			CourseId: 1, StreamId: 7, RecordingId: created.GetRecording().GetId(), Expected: []uint32{1}, TrailingGapMs: 4000,
		}); err != nil {
			t.Fatal(err)
		}
		env.api.handOffs.Wait()
		if want := []string{"file=a", "gap_ms=4000"}; !slices.Equal(*parts, want) {
			t.Errorf("runner got %v, want %v", *parts, want)
		}
	})

	t.Run("the gap of a segment without chunks goes to the next", func(t *testing.T) {
		uploads, parts := recordingRunner(t, http.StatusOK)
		env := newRecordingAPI(t, uploads, courseOneLecture())
		created := env.create(t, "PRES")
		path := created.GetUploadPath()
		env.mustPut(t, path, "0", "0", "a")
		store, _ := env.api.recordingStore()
		// Shared again, and stopped before the recorder handed out a chunk.
		if err := store.setGap(uint(created.GetRecording().GetId()), 1, 1000); err != nil {
			t.Fatal(err)
		}
		if rec := env.put(path, "2", "0?gap_ms=500", "x"); rec.Code != http.StatusOK {
			t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
		}
		if _, err := env.finalize(created.GetRecording().GetId()); err != nil {
			t.Fatal(err)
		}
		if want := []string{"file=a", "gap_ms=1500", "file=x"}; !slices.Equal(*parts, want) {
			t.Errorf("runner got %v, want %v", *parts, want)
		}
	})

	t.Run("a gap that is no number is refused", func(t *testing.T) {
		env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
		path := env.create(t, "PRES").GetUploadPath()
		if rec := env.put(path, "1", "0?gap_ms=soon", "x"); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

func TestBrowserRecordingChunk(t *testing.T) {
	env := newRecordingAPI(t, &fakeUploads{}, courseOneLecture())
	path := env.create(t, "PRES").GetUploadPath()

	for _, tt := range []struct {
		name, path, segment, seq string
		body                     string
		want                     int
	}{
		{"an unknown ticket", browserRecordingPrefix + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "0", "0", "a", http.StatusNotFound},
		{"a malformed ticket", browserRecordingPrefix + "nope", "0", "0", "a", http.StatusNotFound},
		{"a segment out of range", path, "1000", "0", "a", http.StatusBadRequest},
		{"a chunk that is no number", path, "0", "x", "a", http.StatusBadRequest},
		{"a chunk over the limit", path, "0", "0", strings.Repeat("a", maxRecordingChunkSize+1), http.StatusBadRequest},
		{"a chunk", path, "0", "0", "a", http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if rec := env.put(tt.path, tt.segment, tt.seq, tt.body); rec.Code != tt.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
}
