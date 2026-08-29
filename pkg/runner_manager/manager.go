package runner_manager

import (
	"context"
	"errors"
	"fmt"
	log "log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	go_anel_pwrctrl "github.com/RBG-TUM/go-anel-pwrctrl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/pkg/camera"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/voice-service/pb"
)

// Manager manages communication with runners and handles job distribution
type Manager struct {
	dao        dao.DaoWrapper
	listenAddr string
	logger     *log.Logger

	protobuf.UnimplementedRunnerManagerServiceServer

	streamStartLock sync.Mutex
	massStorage     string

	subtitleClient pb.SubtitleGeneratorClient
	subtitleAuth   string

	camService      CamService
	camsHandled     map[uint]bool
	camsHandledLock sync.Mutex

	lightLock sync.Mutex

	liveStateNotifier LiveStateNotifier
	liveNotified      map[uint]bool
	liveNotifiedLock  sync.Mutex
}

type CamService interface {
	For(address string, cameraType model.CameraType) (camera.Cam, error)
}

// LiveStateNotifier is called whenever a stream transitions into or out of the live state,
// so that other parts of the application (e.g. the websocket hub) can react to it.
type LiveStateNotifier func(streamID uint, live bool)

// New returns a new instance of Manager with the given Options
func New(dao dao.DaoWrapper, opts ...Option) *Manager {
	m := Manager{
		dao:        dao,
		listenAddr: ":50056",
		logger: log.New(log.NewJSONHandler(os.Stdout, &log.HandlerOptions{
			Level: log.LevelDebug,
		})).With("service", "runner_manager"),
		streamStartLock: sync.Mutex{},
		camsHandled:     make(map[uint]bool),
		liveNotified:    make(map[uint]bool),
	}
	m.applyOpts(opts)
	return &m
}

// Option is a func that applies configuration to the Manager
type Option func(m *Manager)

func (m *Manager) TriggerDueStreams() error {
	ctx := context.Background()
	streams, err := m.dao.GetDueStreamsForRunners()
	if err != nil {
		return err
	}
	if len(streams) == 0 {
		return nil
	}
	m.logger.Info("Triggering due streams", "count", len(streams))

	var errs []error

	for _, s := range streams {
		lh, err := m.dao.GetLectureHallByID(s.LectureHallID)
		if err != nil {
			errs = append(errs, fmt.Errorf("GetLectureHallByID: %w", err))
			continue
		}
		versions := []protobuf.StreamVersion{
			protobuf.StreamVersion_STREAM_VERSION_COMBINED,
			protobuf.StreamVersion_STREAM_VERSION_PRESENTATION,
			protobuf.StreamVersion_STREAM_VERSION_CAMERA,
		}
		for _, version := range versions {
			// Skip if a runner job already exists for this stream+version (prevents double-triggering)
			exists, err := m.dao.StreamsDao.HasRunnerJobForStreamVersion(s.ID, modelStreamVersion(version))
			if err != nil {
				errs = append(errs, fmt.Errorf("HasRunnerJobForStreamVersion: %w", err))
				continue
			}
			if exists {
				m.logger.Debug("skipping already-triggered stream version", "stream", s.ID, "version", version)
				continue
			}

			runner, client, err := m.getClient(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("getClient: %w", err))
				continue
			}

			resp, err := m.requestStreamVersion(ctx, s, client, lh, version)
			if err != nil && !errors.Is(err, errNotNoLectureSource) {
				errs = append(errs, fmt.Errorf("RequestStream %v: %w", version, err))
				continue
			}
			m.logger.With("stream", s.ID, "job", resp.GetJobId(), "version", version).Info("started Stream")
			if resp.GetJobId() != "" {
				err = m.dao.StreamsDao.SaveRunnerJobForStream(s.ID, modelStreamVersion(version), runner.Hostname, resp.GetJobId())
				if err != nil {
					errs = append(errs, fmt.Errorf("SaveRunnerJobForStream: %w", err))
				}
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to start stream: %v", errs)
	}
	return nil
}

// WithListenAddr sets the address the Manager listens on for gRPC connections from the Runner.
// If not applied, the default is used (:50056)
func WithListenAddr(addr string) Option {
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}
	return func(m *Manager) {
		m.listenAddr = addr
	}
}

func WithMassStorage(path string) Option {
	return func(m *Manager) {
		m.massStorage = path
	}
}

func WithCamService(camService CamService) Option {
	return func(m *Manager) {
		m.camService = camService
	}
}

func WithSubtitleClient(client pb.SubtitleGeneratorClient, auth string) Option {
	return func(m *Manager) {
		m.subtitleClient = client
		m.subtitleAuth = auth
	}
}

// WithLiveStateNotifier registers a callback invoked when a stream starts or stops being live,
// e.g. to notify viewers via websocket.
func WithLiveStateNotifier(notifier LiveStateNotifier) Option {
	return func(m *Manager) {
		m.liveStateNotifier = notifier
	}
}

func (m *Manager) applyOpts(opts []Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *Manager) Run() error {
	lis, err := net.Listen("tcp", m.listenAddr)
	if err != nil {
		return fmt.Errorf("run manager: %v", err)
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	protobuf.RegisterRunnerManagerServiceServer(grpcServer, m)
	reflection.Register(grpcServer)
	go func(listener net.Listener) {
		defer func() {
			_ = listener.Close()
		}()
		if err := grpcServer.Serve(lis); err != nil {
			m.logger.Error("failed to serve runner manager", "err", err)
		}
	}(lis)
	return nil
}

func (m *Manager) Register(ctx context.Context, req *protobuf.RegisterRequest) (*protobuf.RegisterResponse, error) {
	m.logger.Info("Register Runner", "d", req)
	err := m.dao.RunnerDao.Create(ctx, &model.Runner{
		Hostname:       req.GetHostname(),
		Port:           uint32(req.GetPort()),
		TimeOfRegister: time.Now(),
		Version:        req.GetVersion(),
	})
	if err != nil {
		return nil, fmt.Errorf("create runner: %v", err)
	}

	// Clean orphaned jobs from previous runner process — the old job UUIDs no longer exist
	// in the new runner. The stale reaper will detect affected streams and set them not-live.
	if err := m.dao.StreamsDao.ClearRunnerJobsByHostname(req.GetHostname()); err != nil {
		m.logger.Error("failed to clear orphaned runner jobs on re-registration", "hostname", req.GetHostname(), "err", err)
	}

	return &protobuf.RegisterResponse{}, nil
}

func (m *Manager) Notify(ctx context.Context, notification *protobuf.Notification) (*protobuf.NotificationResponse, error) {
	switch notification.Data.(type) {
	case *protobuf.Notification_Heartbeat:
		runner, err := m.dao.RunnerDao.Get(ctx, notification.GetHeartbeat().GetHostname())
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "runner not found: %v", err)
		}
		newDraining := notification.GetHeartbeat().GetDraining()
		newJobCount := notification.GetHeartbeat().GetJobCount()
		if runner.Draining != newDraining || runner.JobCount != newJobCount {
			m.logger.Info("Runner state changed", "hostname", notification.GetHeartbeat().GetHostname(), "draining", newDraining, "jobCount", newJobCount)
		}
		runner.LastSeen = time.Now()
		runner.Draining = newDraining
		runner.JobCount = newJobCount
		err = m.dao.RunnerDao.Update(ctx, &runner)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "update runner: %v", err)
		}
		return &protobuf.NotificationResponse{}, nil
	case *protobuf.Notification_StreamStart:
		return &protobuf.NotificationResponse{}, m.streamStarted(ctx, notification.GetStreamStart())
	case *protobuf.Notification_StreamEnd:
		return &protobuf.NotificationResponse{}, m.streamEnded(ctx, notification.GetStreamEnd())
	case *protobuf.Notification_VodReady:
		return m.handleVODReady(ctx, notification.GetVodReady())
	case *protobuf.Notification_ThumbnailReady:
		return &protobuf.NotificationResponse{}, m.saveThumbnail(ctx, notification.GetThumbnailReady())
	default:
		return nil, status.Error(codes.Unimplemented, "unsupported notification type")
	}
}

func (m *Manager) getClient(ctx context.Context) (model.Runner, protobuf.RunnerServiceClient, error) {
	r, err := m.dao.RunnerDao.ReserveRunner(ctx)
	if err != nil {
		return model.Runner{}, nil, fmt.Errorf("reserve available runner: %w", err)
	}
	conn, err := dialRunner(r)
	if err != nil {
		return model.Runner{}, nil, fmt.Errorf("dial runner: %w", err)
	}
	return r, protobuf.NewRunnerServiceClient(conn), nil
}

// modelStreamVersion maps a protobuf.StreamVersion to the model.StreamVersion used for DB storage.
func modelStreamVersion(version protobuf.StreamVersion) model.StreamVersion {
	switch version {
	case protobuf.StreamVersion_STREAM_VERSION_PRESENTATION:
		return model.PRES
	case protobuf.StreamVersion_STREAM_VERSION_CAMERA:
		return model.CAM
	default:
		return model.COMB
	}
}

func (m *Manager) streamStarted(ctx context.Context, req *protobuf.StreamStartNotification) error {
	// This is usually called in bursts, which introduces a chance for race conditions,
	// where a stream is fetched and overwrites the url that the other requests added.

	stream, err := m.dao.GetStreamByID(ctx, strconv.FormatUint(req.Stream.GetId(), 10))
	if err != nil {
		return err
	}
	m.streamStartLock.Lock()
	switch req.GetStreamVersion() {
	case protobuf.StreamVersion_STREAM_VERSION_COMBINED:
		m.dao.StreamsDao.SaveCOMBURL(&stream, *req.Url)
	case protobuf.StreamVersion_STREAM_VERSION_PRESENTATION:
		m.dao.StreamsDao.SavePRESURL(&stream, *req.Url)
	case protobuf.StreamVersion_STREAM_VERSION_CAMERA:
		m.dao.StreamsDao.SaveCAMURL(&stream, *req.Url)
	}
	m.streamStartLock.Unlock()

	m.notifyLiveState(stream.ID, true)

	err = m.handleCamera(ctx, stream)
	if err != nil {
		log.Error("failed to handle camera", "stream", stream.ID, "err", err)
	}

	err = m.handleLightsOn(stream)
	if err != nil {
		log.Error("failed to turn on lights", "stream", stream.ID, "err", err)
	}

	return nil
}

// notifyLiveState invokes the configured LiveStateNotifier, making sure viewers are only notified
// once per stream when going live, even though streamStarted is called once per stream version.
func (m *Manager) notifyLiveState(streamID uint, live bool) {
	if m.liveStateNotifier == nil {
		return
	}
	if live {
		m.liveNotifiedLock.Lock()
		alreadyNotified := m.liveNotified[streamID]
		m.liveNotified[streamID] = true
		m.liveNotifiedLock.Unlock()
		if alreadyNotified {
			return
		}
	} else {
		m.liveNotifiedLock.Lock()
		delete(m.liveNotified, streamID)
		m.liveNotifiedLock.Unlock()
	}
	m.liveStateNotifier(streamID, live)
}

var errNotNoLectureSource = fmt.Errorf("no source configured for this lecture hall ip")

func (m *Manager) requestStreamVersion(ctx context.Context, s model.Stream, client protobuf.RunnerServiceClient, lh model.LectureHall, version protobuf.StreamVersion) (*protobuf.StreamResponse, error) {
	var ip string
	switch version {
	case protobuf.StreamVersion_STREAM_VERSION_COMBINED:
		ip = lh.CombIP
	case protobuf.StreamVersion_STREAM_VERSION_CAMERA:
		ip = lh.CamIP
	case protobuf.StreamVersion_STREAM_VERSION_PRESENTATION:
		ip = lh.PresIP
	default:
		return nil, fmt.Errorf("invalid stream version %v", version)
	}

	outputOptions := "-c:v libx264 -preset veryfast -tune zerolatency -c:a aac -ar 44100 -b:a 128k -g 60 -bufsize 5000k -maxrate 5000k -b:v 3500k"

	if !s.IsSelfStream() {
		switch lh.StreamProtocol {
		case model.RTSP:
			outputOptions = "-c:a copy -c:v copy -rtsp_transport tcp -preset veryfast -tune zerolatency"
		case model.SRT:
			outputOptions = "-c:a copy -c:v copy -preset veryfast -tune zerolatency"
		}
	}

	var input string
	if !s.IsSelfStream() {
		switch lh.StreamProtocol {
		case model.RTSP:
			input = fmt.Sprintf("rtsp://%s", ip)
		case model.SRT:
			input = fmt.Sprintf("srt://%s", ip)
		default:
			return nil, fmt.Errorf("invalid stream protocol %v", lh.StreamProtocol)
		}
	} else {
		// Lecture is Selfstream
		course, err := m.dao.CoursesDao.GetCourseById(ctx, s.CourseID)
		if err != nil {
			return nil, err
		}
		input = fmt.Sprintf("%s/%s-%d", tools.Cfg.IngestBase, course.Slug, s.ID)
	}
	return client.RequestStream(ctx, &protobuf.StreamRequest{
		StreamId:            ptr.Take(uint64(s.ID)),
		Version:             ptr.Take(version),
		End:                 timestamppb.New(s.End),
		FfmpegOutputOptions: ptr.Take(outputOptions),
		Input:               ptr.Take(input),
	})
}

func (m *Manager) RequestSelfStream(ctx context.Context, stream model.Stream) error {
	// reject streams that are more than 30 minutes in the future or more than 30 minutes past
	if !(time.Now().After(stream.Start.Add(time.Minute*-30)) && time.Now().Before(stream.End.Add(time.Minute*30))) {
		m.logger.Warn("Stream rejected, time out of bounds", "streamId", stream.ID)
		return errors.New("stream rejected")
	}

	runner, client, err := m.getClient(ctx)
	if err != nil {
		m.logger.Error("Could not get client", "err", err)
		return err
	}

	resp, err := m.requestStreamVersion(ctx, stream, client, model.LectureHall{}, protobuf.StreamVersion_STREAM_VERSION_COMBINED)
	if err != nil && !errors.Is(err, errNotNoLectureSource) {
		m.logger.Error("Could not start selfstream", "err", err)
		return err
	}
	m.logger.With("stream", stream.ID, "job", resp.GetJobId(), "version", model.COMB).Info("started Stream")
	if resp.GetJobId() != "" {
		if err := m.dao.StreamsDao.SaveRunnerJobForStream(stream.ID, model.COMB, runner.Hostname, resp.GetJobId()); err != nil {
			m.logger.Error("Could not save runner job for stream", "err", err)
		}
	}
	return nil
}

func (m *Manager) saveThumbnail(ctx context.Context, req *protobuf.ThumbnailReadyNotification) error {
	var fileType model.FileType
	switch req.GetStreamVersion() {
	case protobuf.StreamVersion_STREAM_VERSION_CAMERA:
		fileType = model.FILETYPE_THUMB_LG_CAM
	case protobuf.StreamVersion_STREAM_VERSION_PRESENTATION:
		fileType = model.FILETYPE_THUMB_LG_PRES
	case protobuf.StreamVersion_STREAM_VERSION_COMBINED:
		fileType = model.FILETYPE_THUMB_LG_COMB
	default:
		return status.Errorf(codes.InvalidArgument, "invalid stream version %v", req.GetStreamVersion())
	}

	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(req.Stream.GetId(), 10))
	if err != nil {
		return status.Errorf(codes.NotFound, "can't find stream for id %d: %v", req.Stream.GetId(), err)
	}

	fpath := filepath.Join(m.massStorage, "thumbs", stream.Start.Format("2006/01"), fmt.Sprintf("%d", stream.CourseID))
	fname := fmt.Sprintf("%d_%s.jpeg", stream.ID, req.GetStreamVersion().String())

	file := model.File{
		StreamID: stream.ID,
		// /mass/thumbs/2025/10/500/1024_STREAM_VERSION_COMBINED.jpeg
		Path:     filepath.Join(fpath, fname),
		Filename: fname,
		Type:     fileType,
	}

	err = os.MkdirAll(fpath, 0o755)
	if err != nil {
		return status.Errorf(codes.Internal, "can't make directory: %v", err)
	}

	f, err := os.OpenFile(filepath.Join(fpath, file.Filename), os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return status.Errorf(codes.Internal, "can't open file: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = f.Write(req.GetThumbnail())
	if err != nil {
		return status.Errorf(codes.Internal, "can't write file: %v", err)
	}

	err = m.dao.FileDao.NewFile(&file)
	if err != nil {
		return status.Errorf(codes.Internal, "can't save thumbnail to db: %v", err)
	}

	return nil
}

func (m *Manager) handleVODReady(ctx context.Context, notification *protobuf.VODReadyNotification) (*protobuf.NotificationResponse, error) {
	m.logger.Debug("vodReady", "payload", notification)
	streamId := notification.Stream.GetId()
	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(streamId, 10))
	if err != nil {
		return nil, err
	}
	modelVersion := model.COMB
	switch *notification.StreamVersion {
	case protobuf.StreamVersion_STREAM_VERSION_COMBINED:
		stream.PlaylistUrl = notification.GetUrl()
	case protobuf.StreamVersion_STREAM_VERSION_PRESENTATION:
		modelVersion = model.PRES
		stream.PlaylistUrlPRES = notification.GetUrl()
	case protobuf.StreamVersion_STREAM_VERSION_CAMERA:
		modelVersion = model.CAM
		stream.PlaylistUrlCAM = notification.GetUrl()
	}
	stream.Recording = true
	err = m.dao.StreamsDao.SaveStream(&stream)
	if err != nil {
		return nil, err
	}

	err = m.requestSubtitles(ctx, stream, modelVersion)
	if err != nil {
		log.Error("failed to request subtitles", "stream", streamId, "version", modelVersion, "err", err)
	}

	return &protobuf.NotificationResponse{}, nil
}

func (m *Manager) requestSubtitles(ctx context.Context, stream model.Stream, version model.StreamVersion) error {
	if m.subtitleClient == nil {
		m.logger.Info("skipping subtitle generation, not configured", "stream-id", stream.ID, "version", version)
		return nil // nothing to do
	}
	course, err := m.dao.CoursesDao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return err
	}
	if course.ShouldGenerateSubtitles(version, stream.LectureHallID) {
		outCtx := context.Background()
		if m.subtitleAuth != "" {
			outCtx = metadata.AppendToOutgoingContext(outCtx, "auth", m.subtitleAuth)
		}
		// sign playlist so that subtitle generator can access it
		err = tools.SetSignedPlaylists(&stream, &model.User{}, false)
		if err != nil {
			log.Error("can't sign playlists for subtitle generation", "err", err)
		}
		var url string
		switch version {
		case model.COMB:
			url = stream.PlaylistUrl
		case model.PRES:
			url = stream.PlaylistUrlPRES
		default:
			url = stream.PlaylistUrlCAM
		}
		_, err = m.subtitleClient.Generate(outCtx, &pb.GenerateRequest{
			StreamId:   int32(stream.ID),
			SourceFile: url,
			Language:   course.Language.String,
		})
		return err
	}
	m.logger.Info("skipping subtitle generation, not eligible", "stream-id", stream.ID, "version", version)
	return nil
}

// handleCamera takes care of PTZ camera control and positioning on stream start`
func (m *Manager) handleCamera(ctx context.Context, stream model.Stream) (err error) {
	if stream.IsSelfStream() {
		return nil
	}
	m.camsHandledLock.Lock()
	if m.camsHandled[stream.ID] {
		m.camsHandledLock.Unlock()
		return nil
	}
	m.camsHandled[stream.ID] = true
	m.camsHandledLock.Unlock()

	defer func() {
		if err != nil {
			m.camsHandledLock.Lock()
			m.camsHandled[stream.ID] = false
			m.camsHandledLock.Unlock()
		}
	}()

	lh, err := m.dao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		return err
	}
	course, err := m.dao.CoursesDao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return err
	}
	ctrl, err := m.camService.For(lh.CameraIP, lh.CameraType)
	if err != nil {
		return err
	}
	var pref *model.CameraPresetPreference
	for _, preference := range course.GetCameraPresetPreference() {
		if preference.LectureHallID == stream.LectureHallID {
			pref = &preference
			break
		}
	}

	var p *model.CameraPreset
	for _, preset := range lh.CameraPresets {
		if preset.IsDefault && pref == nil {
			p = &preset
			break
		}
		if pref != nil && preset.PresetID == pref.PresetID {
			p = &preset
			break
		}
	}
	if p != nil {
		return ctrl.SetPreset(p.PresetID)
	}
	return nil
}

// handleLightsOn turns on the lights in the lecture hall when a stream starts
func (m *Manager) handleLightsOn(stream model.Stream) (err error) {
	m.lightLock.Lock()
	defer m.lightLock.Unlock()

	if stream.IsSelfStream() {
		return nil
	}
	lh, err := m.dao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		return err
	}
	client := go_anel_pwrctrl.New(lh.PwrCtrlIp, tools.Cfg.Auths.PwrCrtlAuth)
	for i := range 3 {
		err = errors.Join(err, client.TurnOn(i))
	}
	return err
}

// handleLightsOff turns off the lights in the lecture hall when a stream ends if no other streams are live in the same hall
func (m *Manager) handleLightsOff(stream model.Stream) (err error) {
	m.lightLock.Lock()
	defer m.lightLock.Unlock()

	liveStreamsInLectureHall, err := m.dao.StreamsDao.GetLiveStreamsInLectureHall(stream.LectureHallID)
	if err != nil {
		return err
	}
	if len(liveStreamsInLectureHall) > 1 {
		return nil // another stream is live, don't turn off the light
	}
	if len(liveStreamsInLectureHall) == 1 && liveStreamsInLectureHall[0].ID != stream.ID {
		return nil // the one different live stream is not this one, don't turn off the light
	}
	lectureHall, err := m.dao.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		return err
	}
	client := go_anel_pwrctrl.New(lectureHall.PwrCtrlIp, tools.Cfg.Auths.PwrCrtlAuth)
	for i := range 3 {
		err = errors.Join(err, client.TurnOff(i))
	}
	return err
}

func (m *Manager) streamEnded(ctx context.Context, notification *protobuf.StreamEndNotification) error {
	m.logger.Debug("streamEnd", "payload", notification)
	streamID := uint(notification.GetStream().GetId())
	version := modelStreamVersion(notification.GetStreamVersion())

	// Delete the runner job for this specific version
	if err := m.dao.StreamsDao.ClearRunnerJobForStream(streamID, version); err != nil {
		m.logger.Error("failed to clear runner job for version", "stream", streamID, "version", version, "err", err)
	}

	// Only set stream not-live when all versions have ended
	remaining, err := m.dao.StreamsDao.CountRunnerJobsForStream(streamID)
	if err != nil {
		return fmt.Errorf("count runner jobs: %w", err)
	}
	if remaining > 0 {
		m.logger.Debug("stream version ended, other versions still running", "stream", streamID, "version", version, "remaining", remaining)
		return nil
	}

	if err := m.dao.StreamsDao.SetStreamNotLiveById(streamID); err != nil {
		return err
	}
	m.notifyLiveState(streamID, false)

	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(notification.GetStream().GetId(), 10))
	if err != nil {
		return err
	}

	m.camsHandledLock.Lock()
	delete(m.camsHandled, streamID)
	m.camsHandledLock.Unlock()

	if err := m.handleLightsOff(stream); err != nil {
		log.Error("failed to turn off lights", "stream", stream.ID, "err", err)
	}
	return nil
}

// EndStream cancels all runner jobs tracked for the given stream, e.g. when an admin manually
// stops a stream. It is a no-op if no runner jobs are tracked for the stream (e.g. because it
// is running on a legacy worker instead).
func (m *Manager) EndStream(ctx context.Context, streamID uint, discardVoD bool) error {
	jobs, err := m.dao.StreamsDao.GetRunnerJobsForStream(streamID)
	if err != nil {
		return fmt.Errorf("get runner jobs for stream: %w", err)
	}

	var errs []error
	for _, job := range jobs {
		if err := m.endRunnerJob(ctx, job, discardVoD); err != nil {
			errs = append(errs, err)
			m.logger.Error("could not end runner job", "stream", streamID, "runner", job.RunnerHostname, "job", job.JobID, "err", err)
			continue
		}
		// Only clear the job record if the runner acknowledged the cancellation
		if err := m.dao.StreamsDao.ClearRunnerJobForStream(streamID, job.Version); err != nil {
			errs = append(errs, fmt.Errorf("clear runner job for stream version %s: %w", job.Version, err))
		}
	}

	// Always set stream not-live — this is the authoritative admin action
	if err := m.dao.StreamsDao.SetStreamNotLiveById(streamID); err != nil {
		errs = append(errs, fmt.Errorf("set stream not live: %w", err))
	}
	m.notifyLiveState(streamID, false)

	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(uint64(streamID), 10))
	if err == nil {
		m.camsHandledLock.Lock()
		delete(m.camsHandled, streamID)
		m.camsHandledLock.Unlock()
		if err := m.handleLightsOff(stream); err != nil {
			m.logger.Error("failed to turn off lights on EndStream", "stream", streamID, "err", err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("end stream: %v", errs)
	}
	return nil
}

func (m *Manager) endRunnerJob(ctx context.Context, job model.StreamRunnerJob, discardVoD bool) error {
	runner, err := m.dao.RunnerDao.Get(ctx, job.RunnerHostname)
	if err != nil {
		return fmt.Errorf("get runner %s: %w", job.RunnerHostname, err)
	}
	conn, err := dialRunner(runner)
	if err != nil {
		return fmt.Errorf("dial runner %s: %w", job.RunnerHostname, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client := protobuf.NewRunnerServiceClient(conn)
	_, err = client.RequestStreamEnd(ctx, &protobuf.StreamEndRequest{
		JobId:      ptr.Take(job.JobID),
		DiscardVod: ptr.Take(discardVoD),
	})
	if err != nil {
		return fmt.Errorf("request stream end for job %s: %w", job.JobID, err)
	}
	return nil
}

func dialRunner(runner model.Runner) (*grpc.ClientConn, error) {
	return grpc.NewClient(fmt.Sprintf("%s:%d", runner.Hostname, runner.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// ReapStaleStreams finds streams that are stuck in live state and cleans them up.
// This handles the case where the runner crashes and can never deliver StreamEndNotification.
func (m *Manager) ReapStaleStreams() {
	ctx := context.Background()
	streams, err := m.dao.StreamsDao.GetStaleStreams(ctx)
	if err != nil {
		m.logger.Error("failed to get stale streams", "err", err)
		return
	}
	for _, s := range streams {
		m.logger.Warn("reaping stale stream", "stream", s.ID, "name", s.Name)
		if err := m.dao.StreamsDao.SetStreamNotLiveById(s.ID); err != nil {
			m.logger.Error("failed to set stale stream not-live", "stream", s.ID, "err", err)
		}
		if err := m.dao.StreamsDao.ClearRunnerJobsForStream(s.ID); err != nil {
			m.logger.Error("failed to clear runner jobs for stale stream", "stream", s.ID, "err", err)
		}
		m.notifyLiveState(s.ID, false)

		m.camsHandledLock.Lock()
		delete(m.camsHandled, s.ID)
		m.camsHandledLock.Unlock()

		if err := m.handleLightsOff(s); err != nil {
			m.logger.Error("failed to turn off lights for stale stream", "stream", s.ID, "err", err)
		}
	}
}

func (m *Manager) UpdateLights() {
	res, err := m.dao.GetLiveStateForPwrCtrl()
	if err != nil {
		m.logger.Error("Couldn't get the power control live state.", "Err", err)
		return
	}

	for _, r := range res {
		if r.PwrCtrlIP == "" {
			continue
		}
		client := go_anel_pwrctrl.New(r.PwrCtrlIP, tools.Cfg.Auths.PwrCrtlAuth)
		for i := range 3 {
			if r.NumLive > 0 {
				err = client.TurnOn(i)
				if err != nil {
					m.logger.Warn("Couldn't set the power control to on", "Err", err, "ip", r.PwrCtrlIP)
				}
			} else {
				err = client.TurnOff(i)
				if err != nil {
					m.logger.Warn("Couldn't set the power control to off", "Err", err, "ip", r.PwrCtrlIP)
				}
			}
		}
	}
}
