package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// browserRecordingPrefix is the upload_path createBrowserRecording answers, without the ticket.
const browserRecordingPrefix = "/api/v2/browser-recordings/"

// browserRecordingChunkPath is where a chunk goes, beside the gateway: its body is the
// chunk itself, which an RPC cannot take.
const browserRecordingChunkPath = "PUT " + browserRecordingPrefix + "{ticket}/segments/{segment}/chunks/{seq}"

var (
	errNoMassStorage          = errors.New("no directory configured for uploads")
	errRecordingFinalized     = errors.New("the recording is finalized")
	errRecordingHandedOff     = errors.New("a runner has the recording already")
	errRecordingHandingOff    = errors.New("the recording is being handed to a runner")
	errRecordingEmpty         = errors.New("nothing of the recording has arrived")
	errRecordingIncomplete    = errors.New("chunks of the recording are still missing")
	errUnknownRecordingTicket = errors.New("unknown recording")
	errNoSuchRecording        = errors.New("no such recording")
)

// recordingLock is held for reading while a chunk is committed, and for writing while
// what ends a recording's chunks runs: finalizing it, which decides what the runner
// gets, and throwing it away. Without it a chunk could be answered 200, and dropped
// from the browser, after the hand-off read what arrived or the directory was removed.
// Entries are never removed: a few bytes per recording for the life of the process.
func (a *API) recordingLock(id uint) *sync.RWMutex {
	l, _ := a.recordingLocks.LoadOrStore(id, &sync.RWMutex{})
	return l.(*sync.RWMutex)
}

func (a *API) recordingStore() (recordingStore, error) {
	if a.massStorage == "" {
		return recordingStore{}, e.WithStatus(http.StatusInternalServerError, errNoMassStorage)
	}
	return recordingStore{root: filepath.Join(a.massStorage, "browser-recordings")}, nil
}

// CreateBrowserRecording starts a recording of one version of a lecture. Its ticket,
// like an integration's upload ticket, is kept only as a hash.
func (a *API) CreateBrowserRecording(ctx context.Context, req *protobuf.CreateBrowserRecordingRequest) (*protobuf.CreateBrowserRecordingResponse, error) {
	version := model.StreamVersion(req.GetVersion())
	if version != model.PRES && version != model.CAM {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("version must be PRES or CAM"))
	}
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	// As for an upload: the lecture's own stream would race the recording for the playlist.
	if stream.LiveNow {
		return nil, e.WithStatus(http.StatusConflict, errors.New("the lecture is live"))
	}
	if _, err := a.recordingStore(); err != nil {
		return nil, err
	}
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, err
	}
	mime := req.GetMimeType()
	if len(mime) > 128 {
		mime = mime[:128]
	}

	ticket, hash, err := newUploadTicket()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not create the recording"))
	}
	rec := model.BrowserRecording{
		StreamID: stream.ID, Version: version, MimeType: mime, CreatedByID: user.ID, TicketHash: hash,
	}
	if err := a.dao.BrowserRecordingDao.CreateBrowserRecording(ctx, &rec); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: browser recording %d of %s started", stream.ID, rec.ID, version), forStream(stream))

	pb, err := a.browserRecordingToProto(rec)
	if err != nil {
		return nil, err
	}
	return &protobuf.CreateBrowserRecordingResponse{
		Recording:  pb,
		UploadPath: browserRecordingPrefix + ticket,
	}, nil
}

func (a *API) GetBrowserRecording(ctx context.Context, req *protobuf.GetBrowserRecordingRequest) (*protobuf.BrowserRecording, error) {
	_, rec, err := a.lectureBrowserRecording(ctx, req.GetCourseId(), req.GetStreamId(), req.GetRecordingId())
	if err != nil {
		return nil, err
	}
	return a.browserRecordingToProto(rec)
}

func (a *API) ListBrowserRecordings(ctx context.Context, req *protobuf.ListBrowserRecordingsRequest) (*protobuf.ListBrowserRecordingsResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	recs, err := a.dao.BrowserRecordingDao.ListBrowserRecordings(ctx, stream.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	resp := &protobuf.ListBrowserRecordingsResponse{}
	for _, rec := range recs {
		pb, err := a.browserRecordingToProto(rec)
		if err != nil {
			return nil, err
		}
		resp.Recordings = append(resp.Recordings, pb)
	}
	return resp, nil
}

// FinalizeBrowserRecording ends a recording and hands it to a runner. The browser
// sends what it recorded as expected, so that a chunk still on its way cannot be left
// out; the lecture card sends nothing, to rescue what arrived of a recording whose
// browser is gone.
func (a *API) FinalizeBrowserRecording(ctx context.Context, req *protobuf.FinalizeBrowserRecordingRequest) (*protobuf.BrowserRecording, error) {
	stream, rec, err := a.lectureBrowserRecording(ctx, req.GetCourseId(), req.GetStreamId(), req.GetRecordingId())
	if err != nil {
		return nil, err
	}
	if rec.HandedOffAt != nil {
		return a.browserRecordingToProto(rec)
	}
	store, err := a.recordingStore()
	if err != nil {
		return nil, err
	}

	if rec.FinalizedAt == nil {
		finalized, err := a.finalizeRecording(ctx, store, rec.ID, req.GetExpected())
		switch {
		case errors.Is(err, errRecordingEmpty):
			return nil, e.WithStatus(http.StatusBadRequest, err)
		case errors.Is(err, errRecordingIncomplete):
			return nil, e.WithStatus(http.StatusConflict, err)
		case err != nil:
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		if finalized {
			a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: browser recording %d of %s finalized", stream.ID, rec.ID, rec.Version), forStream(stream))
		}
	} else if rec.HandOffAttempts >= model.MaxBrowserRecordingHandOffAttempts {
		// Asked again after the hand-off gave up: that is the administrator's retry.
		if err := a.dao.BrowserRecordingDao.ResetBrowserRecordingHandOff(ctx, rec.ID); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	}

	rec, err = a.dao.BrowserRecordingDao.GetBrowserRecording(ctx, rec.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.startHandOff(rec)
	return a.browserRecordingToProto(rec)
}

// finalizeRecording ends a recording's chunks, once it has what expected asks for (nothing
// when empty) and anything at all. Under the recording's lock, so that no chunk lands
// after this has seen what arrived. finalized is false when it was finalized already.
// The errors are errRecordingEmpty, errRecordingIncomplete or the store's or database's.
func (a *API) finalizeRecording(ctx context.Context, store recordingStore, id uint, expected []uint32) (finalized bool, err error) {
	lock := a.recordingLock(id)
	lock.Lock()
	defer lock.Unlock()
	received, err := store.received(id)
	if err != nil {
		return false, err
	}
	if !hasChunks(received) {
		return false, errRecordingEmpty
	}
	for i, want := range expected {
		if i >= len(received) || received[i] < want {
			return false, errRecordingIncomplete
		}
	}
	return a.dao.BrowserRecordingDao.FinalizeBrowserRecording(ctx, id, time.Now())
}

// AbandonBrowserRecording throws away what arrived of a recording. Nothing else ever
// deletes chunks a runner does not have: a recording left unfinalized is finalized,
// not removed, see FinalizeIdleBrowserRecordings.
func (a *API) AbandonBrowserRecording(ctx context.Context, req *protobuf.AbandonBrowserRecordingRequest) (*emptypb.Empty, error) {
	stream, rec, err := a.lectureBrowserRecording(ctx, req.GetCourseId(), req.GetStreamId(), req.GetRecordingId())
	if err != nil {
		return nil, err
	}
	if rec.HandedOffAt != nil {
		return nil, e.WithStatus(http.StatusConflict, errRecordingHandedOff)
	}
	store, err := a.recordingStore()
	if err != nil {
		return nil, err
	}
	// Claimed like a hand-off, so that none can start while the chunks are removed.
	if _, busy := a.handingOff.LoadOrStore(rec.ID, struct{}{}); busy {
		return nil, e.WithStatus(http.StatusConflict, errRecordingHandingOff)
	}
	defer a.handingOff.Delete(rec.ID)
	if err := a.dropRecording(ctx, store, rec.ID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: browser recording %d of %s abandoned", stream.ID, rec.ID, rec.Version), forStream(stream))
	return &emptypb.Empty{}, nil
}

// dropRecording deletes a recording and its chunks, under its lock: a chunk committed
// after the check in the handler would otherwise bring the directory back.
func (a *API) dropRecording(ctx context.Context, store recordingStore, id uint) error {
	lock := a.recordingLock(id)
	lock.Lock()
	defer lock.Unlock()
	if err := a.dao.BrowserRecordingDao.DeleteBrowserRecording(ctx, id); err != nil {
		return err
	}
	if err := store.remove(id); err != nil {
		a.log.Warn("can't remove a browser recording's chunks", "recording", id, "err", err)
	}
	return nil
}

// lectureBrowserRecording is a recording of the lecture, which the caller administers.
// One of another lecture is answered like a missing one.
func (a *API) lectureBrowserRecording(ctx context.Context, courseID, streamID, recordingID uint32) (model.Stream, model.BrowserRecording, error) {
	stream, err := a.courseLecture(ctx, courseID, streamID)
	if err != nil {
		return model.Stream{}, model.BrowserRecording{}, err
	}
	rec, err := a.dao.BrowserRecordingDao.GetBrowserRecording(ctx, uint(recordingID))
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && rec.StreamID != stream.ID) {
		return model.Stream{}, model.BrowserRecording{}, e.WithStatus(http.StatusNotFound, errNoSuchRecording)
	}
	if err != nil {
		return model.Stream{}, model.BrowserRecording{}, e.WithStatus(http.StatusInternalServerError, err)
	}
	return stream, rec, nil
}

func (a *API) browserRecordingToProto(rec model.BrowserRecording) (*protobuf.BrowserRecording, error) {
	pb := &protobuf.BrowserRecording{
		Id:            uint32(rec.ID),
		StreamId:      uint32(rec.StreamID),
		Version:       string(rec.Version),
		MimeType:      rec.MimeType,
		Finalized:     rec.FinalizedAt != nil,
		HandedOff:     rec.HandedOffAt != nil,
		HandOffError:  rec.HandOffError,
		HandOffGaveUp: rec.HandedOffAt == nil && rec.HandOffAttempts >= model.MaxBrowserRecordingHandOffAttempts,
		CreatedAt:     timestamppb.New(rec.CreatedAt),
	}
	// Handed off, the chunks are gone.
	if rec.HandedOffAt != nil {
		return pb, nil
	}
	store, err := a.recordingStore()
	if err != nil {
		return nil, err
	}
	if pb.Received, err = store.received(rec.ID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return pb, nil
}

func hasChunks(received []uint32) bool {
	for _, n := range received {
		if n > 0 {
			return true
		}
	}
	return false
}

// handleBrowserRecordingChunk stores one chunk. The ticket is all it is authorized by,
// so a recording goes on when the session that started it expires. It answers only
// once the chunk is on disk: the browser deletes its copy when it gets the answer.
func (a *API) handleBrowserRecordingChunk(w http.ResponseWriter, r *http.Request) {
	hash, ok := uploadTicketHash(r.PathValue("ticket"))
	if !ok {
		writeUploadError(w, e.WithStatus(http.StatusNotFound, errUnknownRecordingTicket))
		return
	}
	segment, err1 := strconv.ParseUint(r.PathValue("segment"), 10, 32)
	seq, err2 := strconv.ParseUint(r.PathValue("seq"), 10, 32)
	if err1 != nil || err2 != nil || segment >= maxRecordingSegments || seq >= maxRecordingChunks {
		writeUploadError(w, e.WithStatus(http.StatusBadRequest, errRecordingLayout))
		return
	}

	rec, err := a.dao.BrowserRecordingDao.GetBrowserRecordingByTicket(r.Context(), hash)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeUploadError(w, e.WithStatus(http.StatusNotFound, errUnknownRecordingTicket))
		return
	}
	if err != nil {
		// The browser keeps the chunk and sends it again.
		a.log.Error("browser recording ticket lookup failed", "err", err)
		writeUploadError(w, e.WithStatus(http.StatusServiceUnavailable, errors.New("try again")))
		return
	}
	if rec.FinalizedAt != nil {
		writeUploadError(w, e.WithStatus(http.StatusConflict, errRecordingFinalized))
		return
	}
	store, err := a.recordingStore()
	if err != nil {
		writeUploadError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRecordingChunkSize)
	staged, err := store.stage(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeUploadError(w, e.WithStatus(http.StatusBadRequest, errors.New("chunk too large (limit is 16 MB)")))
			return
		}
		a.log.Warn("can't receive a browser recording chunk", "recording", rec.ID, "segment", segment, "seq", seq, "err", err)
		writeUploadError(w, e.WithStatus(http.StatusServiceUnavailable, errors.New("the chunk could not be stored, try again")))
		return
	}
	if err := a.commitChunk(r.Context(), store, rec.ID, staged, uint32(segment), uint32(seq)); err != nil {
		writeUploadError(w, err)
		return
	}
	tools.WriteData(w, http.StatusOK, "application/json", []byte("{}"))
}

// commitChunk puts a staged chunk into the recording, if it still takes chunks. Under
// the recording's lock and with the recording read again: it may have been finalized
// or thrown away while the body arrived.
func (a *API) commitChunk(ctx context.Context, store recordingStore, id uint, staged string, segment, seq uint32) error {
	lock := a.recordingLock(id)
	lock.RLock()
	defer lock.RUnlock()
	rec, err := a.dao.BrowserRecordingDao.GetBrowserRecording(ctx, id)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		_ = os.Remove(staged)
		return e.WithStatus(http.StatusNotFound, errUnknownRecordingTicket)
	case err != nil:
		_ = os.Remove(staged)
		a.log.Error("browser recording lookup failed", "recording", id, "err", err)
		return e.WithStatus(http.StatusServiceUnavailable, errors.New("try again"))
	case rec.FinalizedAt != nil:
		_ = os.Remove(staged)
		return e.WithStatus(http.StatusConflict, errRecordingFinalized)
	}
	if err := store.commit(staged, id, segment, seq); err != nil {
		a.log.Error("can't store a browser recording chunk", "recording", id, "segment", segment, "seq", seq, "err", err)
		return e.WithStatus(http.StatusServiceUnavailable, errors.New("the chunk could not be stored, try again"))
	}
	return nil
}
