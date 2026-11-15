package apiv2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/TUM-Dev/gocast/model"
)

// handOffTimeout bounds one hand-off: a lecture's worth of chunks, sent over the
// network between the server and the runners. A variable for the tests.
var handOffTimeout = 2 * time.Hour

// bookkeepingTimeout bounds what is written about a hand-off once it is over. Its own
// context, as the hand-off's may be what ran out.
const bookkeepingTimeout = 30 * time.Second

// markRetryDelay is how long markHandedOff waits before its second attempt, and twice
// that before its third. A variable for the tests.
var markRetryDelay = time.Second

// browserRecordingIdleTimeout is how long a recording that takes no chunks waits for
// its browser before it is finalized with what arrived. Long, so a laptop that is
// offline for a while can still send the rest; finalized rather than removed, because
// a lecture with a video missing its end is better than none.
const browserRecordingIdleTimeout = 72 * time.Hour

// startHandOff hands a finalized recording to a runner in the background, unless that
// is under way already or has given up.
func (a *API) startHandOff(rec model.BrowserRecording) {
	if rec.FinalizedAt == nil || rec.HandedOffAt != nil || rec.HandOffAttempts >= model.MaxBrowserRecordingHandOffAttempts {
		return
	}
	if _, busy := a.handingOff.LoadOrStore(rec.ID, struct{}{}); busy {
		return
	}
	a.handOffs.Add(1)
	go func() {
		defer a.handOffs.Done()
		defer a.handingOff.Delete(rec.ID)
		ctx, cancel := context.WithTimeout(context.Background(), handOffTimeout)
		defer cancel()
		log := a.log.With("recording", rec.ID, "stream", rec.StreamID, "version", rec.Version)
		if err := a.handOff(ctx, rec); err != nil {
			log.Warn("can't hand a browser recording to a runner", "err", err)
			ctx, cancel := context.WithTimeout(context.Background(), bookkeepingTimeout)
			defer cancel()
			if err := a.dao.BrowserRecordingDao.MarkBrowserRecordingHandOffFailed(ctx, rec.ID, err.Error(), !errors.Is(err, errNoRunner)); err != nil {
				log.Error("can't record a failed hand-off", "err", err)
			}
			return
		}
		log.Info("handed a browser recording to a runner")
	}()
}

// handOff sends the recording to a runner as an upload of its version: one file part
// per segment, in order, each the segment's chunks joined. A segment without chunks,
// one whose MediaRecorder never got to a chunk, is left out. The chunks are removed
// only once the runner has the file.
func (a *API) handOff(ctx context.Context, rec model.BrowserRecording) error {
	if a.uploads == nil {
		return errNoRunner
	}
	store, err := a.recordingStore()
	if err != nil {
		return err
	}
	received, err := store.received(rec.ID)
	if err != nil {
		return err
	}
	var segments []uint32
	for seg, n := range received {
		if n > 0 {
			segments = append(segments, uint32(seg))
		}
	}
	if len(segments) == 0 {
		// Finalizing needs chunks, so a finalized recording without any is one whose
		// hand-off went through but could not be recorded, see below: its chunks went
		// with it.
		a.log.Warn("finalized browser recording has no chunks left, taking it as handed off", "recording", rec.ID)
		a.markHandedOff(rec.ID)
		return nil
	}

	slot, err := a.uploads.Reserve(ctx, rec.StreamID, rec.Version)
	if err != nil {
		return fmt.Errorf("%w: %w", errNoRunner, err)
	}
	dropProgress := a.markUploadConverting(rec.StreamID, rec.Version)

	body, contentType := segmentsBody(store, rec.ID, segments, received)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slot.url().String(), body)
	if err != nil {
		_ = body.Close()
		dropProgress()
		return err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		dropProgress()
		return fmt.Errorf("the runner taking the recording can't be reached: %w", err)
	}
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		dropProgress()
		return fmt.Errorf("runner: %s", strings.TrimSpace(string(msg)))
	}

	// The runner has the file. Its chunks go whether or not that can be recorded: left
	// in place they would be handed off again, and imported twice.
	a.markHandedOff(rec.ID)
	if err := store.remove(rec.ID); err != nil {
		a.log.Warn("can't remove a handed-off browser recording", "recording", rec.ID, "err", err)
	}
	return nil
}

// markHandedOff records a hand-off, trying a few times: a database that blinked just
// then should not leave the row to say otherwise.
func (a *API) markHandedOff(id uint) {
	var err error
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * markRetryDelay)
		}
		ctx, cancel := context.WithTimeout(context.Background(), bookkeepingTimeout)
		err = a.dao.BrowserRecordingDao.MarkBrowserRecordingHandedOff(ctx, id, time.Now())
		cancel()
		if err == nil {
			return
		}
	}
	a.log.Error("can't record a hand-off", "recording", id, "err", err)
}

// segmentsBody is the multipart body of the runner's upload endpoint, written as it is
// read: a lecture's chunks are far too large to be put together first.
func segmentsBody(store recordingStore, id uint, segments, received []uint32) (io.ReadCloser, string) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		err := func() error {
			for _, seg := range segments {
				part, err := mw.CreateFormFile("file", fmt.Sprintf("segment-%d", seg))
				if err != nil {
					return err
				}
				r := store.segment(id, seg, received[seg])
				_, err = io.Copy(part, r)
				_ = r.Close()
				if err != nil {
					return err
				}
			}
			return mw.Close()
		}()
		_ = pw.CloseWithError(err)
	}()
	return pr, mw.FormDataContentType()
}

// ResumeBrowserRecordingHandOffs hands off the finalized recordings no runner has yet:
// those a restart cut off and those that failed for a while. Run on boot and by a cron.
func (a *API) ResumeBrowserRecordingHandOffs() {
	recs, err := a.dao.BrowserRecordingDao.ListPendingHandOffs(context.Background())
	if err != nil {
		a.log.Error("can't list browser recordings to hand off", "err", err)
		return
	}
	for _, rec := range recs {
		a.startHandOff(rec)
	}
}

// FinalizeIdleBrowserRecordings finalizes the recordings that have taken no chunk for
// browserRecordingIdleTimeout, and hands off what arrived of them. Run by a cron.
func (a *API) FinalizeIdleBrowserRecordings() {
	ctx := context.Background()
	store, err := a.recordingStore()
	if err != nil {
		return
	}
	// A chunk takes seconds to arrive; one staged an hour ago is left from a crash.
	store.sweepStaging(time.Now().Add(-time.Hour))
	cutoff := time.Now().Add(-browserRecordingIdleTimeout)
	recs, err := a.dao.BrowserRecordingDao.ListUnfinalizedBrowserRecordings(ctx, cutoff)
	if err != nil {
		a.log.Error("can't list idle browser recordings", "err", err)
		return
	}
	for _, rec := range recs {
		log := a.log.With("recording", rec.ID, "stream", rec.StreamID)
		last, err := store.lastWrite(rec.ID)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Warn("can't tell when a browser recording last took a chunk", "err", err)
			continue
		}
		if last.After(cutoff) {
			continue
		}
		dropped, err := a.finalizeIdle(ctx, store, rec.ID)
		if err != nil {
			log.Warn("can't finalize an idle browser recording", "err", err)
			continue
		}
		if dropped {
			continue
		}
		log.Info("finalized an idle browser recording")
		rec, err = a.dao.BrowserRecordingDao.GetBrowserRecording(ctx, rec.ID)
		if err == nil {
			a.startHandOff(rec)
		}
	}
}

// finalizeIdle finalizes a recording with what arrived, or removes it when nothing did:
// started, and nothing ever came. In one go under the recording's lock, so that a chunk
// arriving after all is either part of what is finalized or refused.
func (a *API) finalizeIdle(ctx context.Context, store recordingStore, id uint) (dropped bool, err error) {
	lock := a.recordingLock(id)
	lock.Lock()
	defer lock.Unlock()
	received, err := store.received(id)
	if err != nil {
		return false, err
	}
	if hasChunks(received) {
		_, err = a.dao.BrowserRecordingDao.FinalizeBrowserRecording(ctx, id, time.Now())
		return false, err
	}
	if err := a.dao.BrowserRecordingDao.DeleteBrowserRecording(ctx, id); err != nil {
		return false, err
	}
	_ = store.remove(id)
	return true, nil
}
