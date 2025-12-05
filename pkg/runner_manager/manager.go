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
}

type CamService interface {
	For(address string, cameraType model.CameraType) (camera.Cam, error)
}

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
	}
	m.applyOpts(opts)
	return &m
}

// Option is a func that applies configuration to the Manager
type Option func(m *Manager)

func (m *Manager) TriggerDueStreams() error {
	m.logger.Info("Triggering due streams")
	ctx := context.Background()
	streams, err := m.dao.GetDueStreamsForRunners()

	m.logger.Info(fmt.Sprintf("%d streams to start for runner", len(streams)))
	if err != nil {
		return err
	}

	var errs []error

	for _, s := range streams {
		lh, err := m.dao.GetLectureHallByID(s.LectureHallID)
		if err != nil {
			errs = append(errs, fmt.Errorf("GetLectureHallByID: %w", err))
			continue
		}
		client, err := m.getClient(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("getClient: %w", err))
		}

		resp, err := m.requestStreamVersion(ctx, s, client, lh, protobuf.StreamVersion_STREAM_VERSION_COMBINED)
		if err != nil && !errors.Is(err, errNotNoLectureSource) {
			errs = append(errs, fmt.Errorf("RequestStream COMB: %w", err))
			continue
		}
		m.logger.With("stream", s.ID, "job", resp.GetJobId(), "version", model.COMB).Info("started Stream")

		resp, err = m.requestStreamVersion(ctx, s, client, lh, protobuf.StreamVersion_STREAM_VERSION_PRESENTATION)
		if err != nil && !errors.Is(err, errNotNoLectureSource) {
			errs = append(errs, fmt.Errorf("RequestStream PRES: %w", err))
			continue
		}
		m.logger.With("stream", s.ID, "job", resp.GetJobId(), "version", model.PRES).Info("started Stream")

		resp, err = m.requestStreamVersion(ctx, s, client, lh, protobuf.StreamVersion_STREAM_VERSION_CAMERA)
		if err != nil && !errors.Is(err, errNotNoLectureSource) {
			errs = append(errs, fmt.Errorf("RequestStream CAM: %w", err))
			continue
		}
		m.logger.With("stream", s.ID, "job", resp.GetJobId(), "version", model.CAM).Info("started Stream")
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
	return &protobuf.RegisterResponse{}, nil
}

func (m *Manager) Notify(ctx context.Context, notification *protobuf.Notification) (*protobuf.NotificationResponse, error) {
	switch notification.Data.(type) {
	case *protobuf.Notification_Heartbeat:
		m.logger.Debug("Heartbeat", "d", notification)
		runner, err := m.dao.RunnerDao.Get(ctx, notification.GetHeartbeat().GetHostname())
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "runner not found: %v", err)
		}
		runner.LastSeen = time.Now()
		runner.Draining = notification.GetHeartbeat().GetDraining()
		runner.JobCount = notification.GetHeartbeat().GetJobCount()
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

func (m *Manager) getClient(ctx context.Context) (protobuf.RunnerServiceClient, error) {
	r, err := m.dao.RunnerDao.ReserveRunner(ctx)
	if err != nil {
		return nil, fmt.Errorf("reserve available runner: %w", err)
	}
	conn, err := dialRunner(r)
	if err != nil {
		return nil, fmt.Errorf("dial runner: %w", err)
	}
	return protobuf.NewRunnerServiceClient(conn), nil
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

	client, err := m.getClient(ctx)
	if err != nil {
		m.logger.Error("Could not get client", "err", err)
		return err
	}

	resp, err := m.requestStreamVersion(ctx, stream, client, model.LectureHall{}, protobuf.StreamVersion_STREAM_VERSION_COMBINED)
	if err != nil && !errors.Is(err, errNotNoLectureSource) {
		m.logger.Error("Could not start selfstream", "err", err)
		return err
	}
	m.logger.With("stream", stream.ID, "job", resp.JobId, "version", model.COMB).Info("started Stream")
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
	err := m.dao.StreamsDao.SetStreamNotLiveById(uint(notification.GetStream().GetId()))
	if err != nil {
		return err
	}

	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(notification.GetStream().GetId(), 10))
	if err != nil {
		return err
	}

	err = m.handleLightsOff(stream)
	if err != nil {
		log.Error("failed to turn on lights", "stream", stream.ID, "err", err)
	}
	return nil
}

func dialRunner(runner model.Runner) (*grpc.ClientConn, error) {
	return grpc.NewClient(fmt.Sprintf("%s:%d", runner.Hostname, runner.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
}
