package runner

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/actions"
)

// pendingVoD is a recording whose VoD a previous run of the runner did not get to.
type pendingVoD struct {
	actions.PendingVoD
	recordingDir string
}

// findPendingVoDs returns the recordings in segmentPath that carry an
// actions.VoDPendingFile, and an error for each marker that can't be used.
func findPendingVoDs(segmentPath string) ([]pendingVoD, []error) {
	markers, err := filepath.Glob(filepath.Join(segmentPath, "*", "*", actions.VoDPendingFile))
	if err != nil {
		return nil, []error{err}
	}
	var pending []pendingVoD
	var errs []error
	for _, m := range markers {
		p, err := actions.ReadPendingVoD(m)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// The VoD is made of the recording the marker sits in, and reported for the stream
		// it names. A marker moved by hand must not make one lecture's VoD of another's.
		dir := filepath.Dir(m)
		if filepath.Base(dir) != p.StreamVersion || filepath.Base(filepath.Dir(dir)) != strconv.FormatUint(p.StreamID, 10) {
			errs = append(errs, fmt.Errorf("%s names stream %d %s, not its directory", m, p.StreamID, p.StreamVersion))
			continue
		}
		pending = append(pending, pendingVoD{PendingVoD: p, recordingDir: dir})
	}
	return pending, errs
}

// resumePendingVoDs makes the VoDs a previous run of the runner left unmade, typically
// because it was restarted while the storage was down. It must run before the runner
// takes jobs, so none of the recordings it finds belongs to a job of this run.
func (r *Runner) resumePendingVoDs() {
	pending, errs := findPendingVoDs(config.Config.SegmentPath)
	for _, err := range errs {
		r.log.Error("can't resume pending VoD", "error", err)
	}
	for _, p := range pending {
		data := map[string]any{
			"streamID":      p.StreamID,
			"streamVersion": p.StreamVersion,
			"recordingDir":  p.recordingDir,
			"detectSilence": p.DetectSilence,
			"drained":       r.drainedCh(),
		}
		logger := r.log.With("stream_id", p.StreamID, "stream_version", p.StreamVersion)
		jID := r.RunAction(nil, vodActions(p.DetectSilence), data, logger)
		logger.Info("resuming pending VoD", "ID", jID, "dir", p.recordingDir)
	}
}
