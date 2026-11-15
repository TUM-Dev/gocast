package apiv2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// errVersionBusy is a hand-off that waits for another import of the same version of the
// lecture. Imports of a version share its progress row, so the end of one could be taken
// for the end of the other, and the chunks of the one still converting removed.
var errVersionBusy = errors.New("another video of this version is being converted")

// handOffTimeout bounds one hand-off: a lecture's worth of chunks, sent over the
// network between the server and the runners. A variable for the tests.
var handOffTimeout = 2 * time.Hour

// bookkeepingTimeout bounds what is written about a hand-off once it is over. Its own
// context, as the hand-off's may be what ran out.
const bookkeepingTimeout = 30 * time.Second

// markRetryDelay is how long markHandedOff waits before its second attempt, growing by
// as much with every attempt up to a minute. A variable for the tests.
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
			// Neither a runner being away nor another import says anything about the file.
			counts := !errors.Is(err, errNoRunner) && !errors.Is(err, errVersionBusy)
			if err := a.dao.BrowserRecordingDao.MarkBrowserRecordingHandOffFailed(ctx, rec.ID, err.Error(), counts); err != nil {
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
		return errRecordingEmpty
	}
	slot, dropProgress, err := a.claimVersion(ctx, rec)
	if err != nil {
		return err
	}

	all, err := store.segments(rec.ID)
	if err != nil {
		dropProgress()
		return err
	}
	body, contentType := segmentsBody(store, rec.ID, all, received)
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

	// The runner has the file, but only on its disk and in memory: a restart or a failed
	// import loses it. The chunks stay until the VOD is there; see
	// CheckBrowserRecordingConversions.
	a.markHandedOff(rec.ID)
	return nil
}

// markHandedOff records a hand-off. Until it is, the cron would hand the recording off
// again and the runner import it twice, so it keeps trying for an hour, while the
// hand-off's claim on the recording keeps another from starting.
func (a *API) markHandedOff(id uint) {
	var err error
	for attempt := range 60 {
		if attempt > 0 {
			time.Sleep(min(time.Duration(attempt)*markRetryDelay, time.Minute))
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

// segmentsBody is the multipart body of the runner's upload endpoint: the segments with
// chunks, each as a file part after a gap_ms field when recording was interrupted before
// it, and a gap_ms field after the last for the time missed at the end. Its body is, written as it is
// read: a lecture's chunks are far too large to be put together first.
func segmentsBody(store recordingStore, id uint, segments, received []uint32) (io.ReadCloser, string) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		err := func() error {
			// The gap of a segment without chunks adds to the next one's, and what is left
			// after the last file is the time missed at the end, sent last.
			var gap int64
			writeGap := func() error {
				if gap <= 0 {
					return nil
				}
				err := mw.WriteField("gap_ms", strconv.FormatInt(gap, 10))
				gap = 0
				return err
			}
			for _, seg := range segments {
				gap += store.gap(id, seg)
				if uint64(seg) >= uint64(len(received)) || received[seg] == 0 {
					continue
				}
				if err := writeGap(); err != nil {
					return err
				}
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
			gap += store.trailingGap(id)
			if err := writeGap(); err != nil {
				return err
			}
			return mw.Close()
		}()
		_ = pw.CloseWithError(err)
	}()
	return pr, mw.FormDataContentType()
}

// lostConversionAfter is how long a hand-off may go without the runner reporting on its
// conversion before it is taken to be lost with a runner that restarted, which keeps
// nothing of an import. Far above what a lecture takes to transcode.
const lostConversionAfter = 6 * time.Hour

// CheckBrowserRecordingConversions removes the chunks of the recordings whose VOD is
// there, which the runner's VODReady marks, and of recordings that are gone, with their
// lecture too. It hands off again the ones whose conversion the runner never reported
// on, and then the hand-offs that are due. A failed conversion is handed back by its
// VODFailed. Run on boot and by a cron.
func (a *API) CheckBrowserRecordingConversions() {
	ctx := context.Background()
	store, err := a.recordingStore()
	if err != nil {
		return
	}
	ids, err := store.recordings()
	if err != nil {
		a.log.Error("can't list browser recordings on disk", "err", err)
	}
	for _, id := range ids {
		rec, err := a.dao.BrowserRecordingDao.GetBrowserRecording(ctx, id)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			a.log.Warn("can't look up a browser recording", "recording", id, "err", err)
			continue
		}
		if err == nil && rec.ConvertedAt == nil {
			a.dropIfLectureGone(ctx, store, rec)
			continue
		}
		if err := store.remove(id); err != nil {
			a.log.Warn("can't remove a browser recording's chunks", "recording", id, "err", err)
		}
	}

	recs, err := a.dao.BrowserRecordingDao.ListConvertingBrowserRecordings(ctx)
	if err != nil {
		a.log.Error("can't list browser recordings being converted", "err", err)
		return
	}
	for _, rec := range recs {
		if time.Since(*rec.HandedOffAt) < lostConversionAfter {
			continue
		}
		log := a.log.With("recording", rec.ID, "stream", rec.StreamID, "version", rec.Version)
		log.Warn("no word of a browser recording's conversion, handing it off again")
		// Still shown as converting, which would keep the next hand-off waiting.
		if err := a.dao.StreamsDao.RemoveTranscodingProgress(rec.Version, rec.StreamID); err != nil {
			log.Warn("can't clear a lost conversion's progress", "err", err)
		}
		if err := a.dao.BrowserRecordingDao.RequeueBrowserRecordingHandOff(ctx, rec.ID, "the conversion did not finish"); err != nil {
			log.Warn("can't take back a lost hand-off", "err", err)
		}
	}
	a.ResumeBrowserRecordingHandOffs()
}

// dropIfLectureGone deletes a recording whose lecture was deleted: its VODReady would be
// dropped with the lecture, so it would never be marked converted, and its chunks would
// stay. Claimed like a hand-off, which it must not cut short.
func (a *API) dropIfLectureGone(ctx context.Context, store recordingStore, rec model.BrowserRecording) {
	_, err := a.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(uint64(rec.StreamID), 10))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if _, busy := a.handingOff.LoadOrStore(rec.ID, struct{}{}); busy {
		return
	}
	defer a.handingOff.Delete(rec.ID)
	a.log.Info("removing a browser recording of a deleted lecture", "recording", rec.ID, "stream", rec.StreamID)
	if err := a.dropRecording(ctx, store, rec.ID); err != nil {
		a.log.Warn("can't remove a browser recording of a deleted lecture", "recording", rec.ID, "err", err)
	}
}

// browserRecordingConverting tells whether a browser recording of the lecture's version
// is finalized and its VOD not there yet: waiting for a runner, or being converted.
func (a *API) browserRecordingConverting(ctx context.Context, streamID uint, version model.StreamVersion) (bool, error) {
	// A browser records a presentation and a camera version, never a combined one.
	if version != model.PRES && version != model.CAM {
		return false, nil
	}
	recs, err := a.dao.BrowserRecordingDao.ListBrowserRecordings(ctx, streamID)
	if err != nil {
		return false, err
	}
	for _, rec := range recs {
		if rec.Version == version && rec.FinalizedAt != nil {
			return true, nil
		}
	}
	return false, nil
}

// claimVersion names the hand-off, reserves a runner for it and has the card show the
// version converting, unless another import of the version is under way: the one to
// finish last would make the VOD, whichever it is. Under a lock per version, so that two
// hand-offs of it can't both find it free.
func (a *API) claimVersion(ctx context.Context, rec model.BrowserRecording) (slot UploadTarget, dropProgress func(), err error) {
	key := fmt.Sprintf("%d/%s", rec.StreamID, rec.Version)
	l, _ := a.versionLocks.LoadOrStore(key, &sync.Mutex{})
	lock := l.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	progress, err := a.dao.StreamsDao.GetTranscodingProgressByVersion(rec.Version, rec.StreamID)
	if err == nil && !progress.Failed {
		return UploadTarget{}, nil, errVersionBusy
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return UploadTarget{}, nil, err
	}
	importID := fmt.Sprintf("browser-%d-%s", rec.ID, hex.EncodeToString(suffix))
	if err := a.dao.BrowserRecordingDao.SetBrowserRecordingImport(ctx, rec.ID, importID); err != nil {
		return UploadTarget{}, nil, fmt.Errorf("name the hand-off: %w", err)
	}
	slot, err = a.uploads.Reserve(ctx, rec.StreamID, rec.Version, importID)
	if err != nil {
		return UploadTarget{}, nil, fmt.Errorf("%w: %w", errNoRunner, err)
	}
	return slot, a.markUploadConverting(rec.StreamID, rec.Version), nil
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
