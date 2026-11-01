package runner_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// VersionSource is one version of a VOD and the signed playlist the runner reads it from.
type VersionSource struct {
	Version     model.StreamVersion
	PlaylistURL string
}

// CutRequest describes the cut of one version of a VOD. TargetStreamID is SourceStreamID to
// replace the VOD, and another lecture to give it the result. PlaylistURL must be signed.
type CutRequest struct {
	SourceStreamID uint
	TargetStreamID uint
	Version        model.StreamVersion
	PlaylistURL    string
	Discard        []model.CutRange
}

// RequestWaveform asks a runner for the audio level timeline of a stream and the length of
// the given versions. The answer is a WaveformReadyNotification, which saveWaveform stores.
func (m *Manager) RequestWaveform(ctx context.Context, streamID uint, sources []VersionSource) (err error) {
	ctx, span := startSpan(ctx, "manager.request_waveform", streamAttrs(uint64(streamID), protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED)...)
	defer endSpan(span, &err)
	runner, client, conn, err := m.getClient(ctx)
	if err != nil {
		return fmt.Errorf("getClient: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(ctx, runnerDispatchTimeout)
	defer cancel()

	req := &protobuf.WaveformRequest{StreamId: ptr.Take(uint64(streamID))}
	for _, src := range sources {
		req.Sources = append(req.Sources, &protobuf.WaveformRequest_Source{
			Version: ptr.Take(protobufStreamVersion(src.Version)), PlaylistUrl: ptr.Take(src.PlaylistURL),
		})
	}
	resp, err := client.RequestWaveform(ctx, req)
	if err != nil {
		return fmt.Errorf("RequestWaveform: %w", err)
	}
	m.logger.With("stream", streamID, "job", resp.GetJobId(), "runner", runner.Hostname).Info("requested waveform")
	return nil
}

// RequestCut asks a runner to cut one version of a VOD. It reports like an upload: progress,
// then VODReady, or VODFailed.
func (m *Manager) RequestCut(ctx context.Context, req CutRequest) (err error) {
	version := protobufStreamVersion(req.Version)
	ctx, span := startSpan(withStreamTrace(ctx, uint64(req.TargetStreamID), version), "manager.request_cut", streamAttrs(uint64(req.TargetStreamID), version)...)
	defer endSpan(span, &err)
	runner, client, conn, err := m.getClient(ctx)
	if err != nil {
		return fmt.Errorf("getClient: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(ctx, runnerDispatchTimeout)
	defer cancel()

	pb := &protobuf.CutVODRequest{
		StreamId:       ptr.Take(uint64(req.SourceStreamID)),
		TargetStreamId: ptr.Take(uint64(req.TargetStreamID)),
		Version:        ptr.Take(version),
		PlaylistUrl:    ptr.Take(req.PlaylistURL),
	}
	for _, r := range req.Discard {
		pb.Discards = append(pb.Discards, &protobuf.CutVODRequest_Range{Start: ptr.Take(r.Start), End: ptr.Take(r.End)})
	}
	resp, err := client.CutVOD(ctx, pb)
	if err != nil {
		return fmt.Errorf("CutVOD: %w", err)
	}
	m.logger.With("stream", req.TargetStreamID, "source", req.SourceStreamID, "version", req.Version, "job", resp.GetJobId(), "runner", runner.Hostname).
		Info("requested cut")
	return nil
}

// saveWaveform stores the timeline as the stream's file of type FILETYPE_WAVEFORM. The file
// is replaced whole, so a repeated notification is harmless, see saveThumbnail.
func (m *Manager) saveWaveform(ctx context.Context, req *protobuf.WaveformReadyNotification) error {
	if len(req.GetPeaks()) == 0 || req.GetBucketSeconds() == 0 {
		return status.Errorf(codes.InvalidArgument, "waveform of %d peaks, %d seconds each", len(req.GetPeaks()), req.GetBucketSeconds())
	}
	streamID := strconv.FormatUint(req.GetStream().GetId(), 10)
	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, streamID)
	if err != nil {
		return lookupError(err, "stream %s", streamID)
	}

	waveform := model.Waveform{
		BucketSeconds: req.GetBucketSeconds(),
		Peaks:         req.GetPeaks(),
		Durations:     map[model.StreamVersion]float64{},
	}
	for _, d := range req.GetDurations() {
		waveform.Durations[modelStreamVersion(d.GetStreamVersion())] = d.GetSeconds()
	}

	dir := filepath.Join(m.massStorage, "waveforms", strconv.FormatUint(uint64(stream.CourseID), 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return status.Errorf(codes.Internal, "can't make directory: %v", err)
	}
	file := model.File{
		StreamID: stream.ID,
		Path:     filepath.Join(dir, fmt.Sprintf("%d.json", stream.ID)),
		Filename: fmt.Sprintf("%d.json", stream.ID),
		Type:     model.FILETYPE_WAVEFORM,
	}
	err = writeFileAtomic(file.Path, func(w io.Writer) error { return json.NewEncoder(w).Encode(waveform) })
	if err != nil {
		return status.Errorf(codes.Internal, "can't write file: %v", err)
	}
	if err := m.dao.FileDao.SetStreamFile(stream.ID, file); err != nil {
		return status.Errorf(codes.Internal, "can't save waveform to db: %v", err)
	}
	return nil
}

// cutVersions are the versions a lecture's VODs come in.
var cutVersions = []model.StreamVersion{model.COMB, model.CAM, model.PRES}

// afterVODReady is what a new VOD of the stream means for cuts, called when its VODReady
// has been taken in.
//
// The waveform describes the VOD before it, so it goes. A cut under way is finished once
// no version of its lecture is still being converted. A version that failed keeps its row,
// and with it the cut open: finishing would move the lecture's data onto a timeline that
// only some of the versions are on.
func (m *Manager) afterVODReady(ctx context.Context, streamID uint) error {
	if err := m.dao.FileDao.DeleteStreamFiles(streamID, model.FILETYPE_WAVEFORM); err != nil {
		return status.Errorf(codes.Internal, "can't drop the waveform: %v", err)
	}

	cut, err := m.dao.LectureCutDao.GetLectureCut(streamID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return status.Errorf(codes.Internal, "can't look up the cut: %v", err)
	}
	for _, v := range cutVersions {
		_, err := m.dao.StreamsDao.GetTranscodingProgressByVersion(v, streamID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Errorf(codes.Internal, "can't look up the transcoding progress: %v", err)
		}
	}

	finished, err := m.dao.LectureCutDao.FinishLectureCut(cut)
	if err != nil {
		return status.Errorf(codes.Internal, "can't finish the cut: %v", err)
	}
	if !finished {
		// The last two versions were ready together and the other one finished the cut.
		return nil
	}
	m.logger.Info("cut finished", "stream", streamID, "source", cut.SourceStreamID)
	// Its own step, after the lecture is on the new timeline: a scan that reported before
	// would have its silences moved a second time.
	if err := m.requestSilenceScan(ctx, streamID); err != nil {
		m.logger.Error("can't request the silence scan of a cut lecture", "stream", streamID, "err", err)
	}
	return nil
}

// requestSilenceScan has a runner scan the best version of the stream for silences.
func (m *Manager) requestSilenceScan(ctx context.Context, streamID uint) error {
	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(uint64(streamID), 10))
	if err != nil {
		return err
	}
	if err := tools.SetSignedPlaylists(&stream, &model.User{}, false); err != nil {
		return fmt.Errorf("sign playlists: %w", err)
	}
	for _, v := range []struct {
		version model.StreamVersion
		url     string
	}{{model.COMB, stream.PlaylistUrl}, {model.CAM, stream.PlaylistUrlCAM}, {model.PRES, stream.PlaylistUrlPRES}} {
		if v.url == "" {
			continue
		}
		return m.RequestRedoPostProcessing(ctx, RedoPostProcessingRequest{
			StreamID: streamID, Version: v.version, PlaylistURL: v.url, DetectSilence: true,
		})
	}
	return nil
}
