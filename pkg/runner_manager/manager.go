package runner_manager

import (
	"context"
	"fmt"
	log "log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

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
}

// New returns a new instance of Manager with the given Options
func New(dao dao.DaoWrapper, opts ...Option) *Manager {
	m := Manager{dao: dao, listenAddr: ":50056"}
	m.applyOpts(opts)
	return &m
}

// Option is a func that applies configuration to the Manager
type Option func(m *Manager)

func (m *Manager) TriggerDueStreams() error {
	log.Info("Triggering due streams")
	ctx := context.Background()
	streams, err := m.dao.GetDueStreamsForRunners()
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
		resp, err := client.RequestStream(ctx, &protobuf.StreamRequest{
			StreamId:            ptr.Take(uint64(s.ID)),
			Version:             ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED),
			End:                 timestamppb.New(s.End),
			FfmpegOutputOptions: ptr.Take("-c:a copy -c:v copy"),
			Input:               ptr.Take(fmt.Sprintf("srt://%s", lh.CombIP)),
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("RequestStream: %w", err))
			continue
		}
		log.With("stream", s.ID, "job", resp.JobId).Info("started Stream")
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
		defer listener.Close()
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
		log.Info("received stream end from runner")
		// passing for now, not implemented.
		return &protobuf.NotificationResponse{}, nil
	case *protobuf.Notification_VodReady:
		log.Info("vodReady", "payload", notification.GetVodReady())
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
	stream, err := m.dao.GetStreamByID(ctx, strconv.FormatUint(req.Stream.GetId(), 10))
	if err != nil {
		return err
	}
	m.dao.StreamsDao.SaveCOMBURL(&stream, *req.Url)
	return nil
}

func dialRunner(ctx context.Context, runner model.Runner) (*grpc.ClientConn, error) {
	return grpc.NewClient(fmt.Sprintf("%s:%d", runner.Hostname, runner.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
}
