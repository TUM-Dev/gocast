package runner_manager

import (
	"context"
	"errors"
	"fmt"
	log "log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TUM-Dev/gocast/web"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// Manager manages communication with runners and handles job distribution
type Manager struct {
	dao        dao.DaoWrapper
	listenAddr string

	protobuf.UnimplementedRunnerManagerServiceServer

	streamStartLock sync.Mutex
}

// New returns a new instance of Manager with the given Options
func New(dao dao.DaoWrapper, opts ...Option) *Manager {
	m := Manager{dao: dao, listenAddr: ":50056", streamStartLock: sync.Mutex{}}
	m.applyOpts(opts)
	return &m
}

// Option is a func that applies configuration to the Manager
type Option func(m *Manager)

func (m *Manager) TriggerDueStreams() error {
	log.Info("Triggering due streams")
	ctx := context.Background()
	streams, err := m.dao.GetDueStreamsForRunners()
	// TODO: Remove this when turning off workers
	if web.VersionTag == "development" {
		workerStreams := m.dao.GetDueStreamsForWorkers()
		streams = append(streams, workerStreams...)
	}

	log.Info(fmt.Sprintf("%d streams to start for runner", len(streams)))
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
		log.With("stream", s.ID, "job", resp.GetJobId(), "version", model.COMB).Info("started Stream")

		resp, err = m.requestStreamVersion(ctx, s, client, lh, protobuf.StreamVersion_STREAM_VERSION_PRESENTATION)
		if err != nil && !errors.Is(err, errNotNoLectureSource) {
			errs = append(errs, fmt.Errorf("RequestStream PRES: %w", err))
			continue
		}
		log.With("stream", s.ID, "job", resp.GetJobId(), "version", model.PRES).Info("started Stream")

		resp, err = m.requestStreamVersion(ctx, s, client, lh, protobuf.StreamVersion_STREAM_VERSION_CAMERA)
		if err != nil && !errors.Is(err, errNotNoLectureSource) {
			errs = append(errs, fmt.Errorf("RequestStream CAM: %w", err))
			continue
		}
		log.With("stream", s.ID, "job", resp.GetJobId(), "version", model.CAM).Info("started Stream")
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
			log.Error("failed to serve runner manager", "err", err)
		}
	}(lis)
	return nil
}

func (m *Manager) Register(ctx context.Context, req *protobuf.RegisterRequest) (*protobuf.RegisterResponse, error) {
	log.Info("Register Runner", "d", req)
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
		log.Debug("Heartbeat", "d", notification)
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
		log.Debug("streamEnd", "payload", notification.GetStreamEnd())
		err := m.dao.StreamsDao.SetStreamNotLiveById(uint(notification.GetStreamEnd().GetStream().GetId()))
		if err != nil {
			return nil, err
		}
		return &protobuf.NotificationResponse{}, nil
	case *protobuf.Notification_VodReady:
		log.Debug("vodReady", "payload", notification.GetVodReady())
		streamId := notification.GetVodReady().Stream.GetId()
		stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(streamId, 10))
		if err != nil {
			return nil, err
		}
		switch *notification.GetVodReady().StreamVersion {
		case protobuf.StreamVersion_STREAM_VERSION_COMBINED:
			stream.PlaylistUrl = notification.GetVodReady().GetUrl()
		case protobuf.StreamVersion_STREAM_VERSION_PRESENTATION:
			stream.PlaylistUrlPRES = notification.GetVodReady().GetUrl()
		case protobuf.StreamVersion_STREAM_VERSION_CAMERA:
			stream.PlaylistUrlCAM = notification.GetVodReady().GetUrl()
		}
		stream.Recording = true
		err = m.dao.StreamsDao.SaveStream(&stream)
		if err != nil {
			return nil, err
		}
		return &protobuf.NotificationResponse{}, nil
	default:
		return nil, status.Error(codes.Unimplemented, "unsupported notification type")
	}
}

func (m *Manager) getClient(ctx context.Context) (protobuf.RunnerServiceClient, error) {
	r, err := m.dao.RunnerDao.GetAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("get Available Runner: %w", err)
	}
	conn, err := dialRunner(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("dialRunner: %w", err)
	}
	return protobuf.NewRunnerServiceClient(conn), nil
}

func (m *Manager) streamStarted(ctx context.Context, req *protobuf.StreamStartNotification) error {
	// This is usually called in bursts, which introduces a chance for race conditions,
	// where a stream is fetched and overwrites the url that the other requests added.
	m.streamStartLock.Lock()
	defer m.streamStartLock.Unlock()

	stream, err := m.dao.GetStreamByID(ctx, strconv.FormatUint(req.Stream.GetId(), 10))
	if err != nil {
		return err
	}
	switch req.GetStreamVersion() {
	case protobuf.StreamVersion_STREAM_VERSION_COMBINED:
		m.dao.StreamsDao.SaveCOMBURL(&stream, *req.Url)
	case protobuf.StreamVersion_STREAM_VERSION_PRESENTATION:
		m.dao.StreamsDao.SavePRESURL(&stream, *req.Url)
	case protobuf.StreamVersion_STREAM_VERSION_CAMERA:
		m.dao.StreamsDao.SaveCAMURL(&stream, *req.Url)
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

	var outputOptions = "-c:v libx264 -preset veryfast -c:a aac -ar 44100 -b:a 128k -b:v 5000k"
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
		input = fmt.Sprintf("rtmp://localhost/%s", course.Slug)
	}
	return client.RequestStream(ctx, &protobuf.StreamRequest{
		StreamId:            ptr.Take(uint64(s.ID)),
		Version:             ptr.Take(version),
		End:                 timestamppb.New(s.End),
		FfmpegOutputOptions: ptr.Take(outputOptions),
		Input:               ptr.Take(input),
	})
}

func dialRunner(ctx context.Context, runner model.Runner) (*grpc.ClientConn, error) {
	return grpc.NewClient(fmt.Sprintf("%s:%d", runner.Hostname, runner.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
}
