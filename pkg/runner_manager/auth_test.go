package runner_manager

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/auth"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

const testToken = "runner-secret"

// serveManager starts m on a loopback port with the production server options and returns
// a client factory, so the tests exercise the real interceptor chain, not the handlers alone.
func serveManager(t *testing.T, m *Manager) func(token, hostname string) protobuf.RunnerManagerServiceClient {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(m.serverOptions()...)
	protobuf.RegisterRunnerManagerServiceServer(srv, m)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return func(token, hostname string) protobuf.RunnerManagerServiceClient {
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		if token != "" {
			opts = append(opts, grpc.WithUnaryInterceptor(auth.UnaryClientInterceptor(token, hostname)))
		}
		conn, err := grpc.NewClient(lis.Addr().String(), opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return protobuf.NewRunnerManagerServiceClient(conn)
	}
}

func newTestManager(t *testing.T, d dao.DaoWrapper) *Manager {
	t.Helper()
	m := New(d, WithRunnerToken(testToken))
	m.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return m
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestRunRefusesWithoutToken(t *testing.T) {
	m := New(dao.DaoWrapper{}, WithListenAddr("127.0.0.1:0"))
	if err := m.Run(); err == nil {
		t.Fatal("Run must fail without a runner token")
	}
}

// Without the token nothing reaches a handler, so the DAO mocks expect no calls at all.
func TestUnauthenticatedCallsAreRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newTestManager(t, dao.DaoWrapper{RunnerDao: mock_dao.NewMockRunnerDao(ctrl), StreamsDao: mock_dao.NewMockStreamsDao(ctrl)})
	client := serveManager(t, m)
	ctx := context.Background()

	for name, c := range map[string]protobuf.RunnerManagerServiceClient{
		"no token":    client("", ""),
		"wrong token": client("nope", "r1"),
		"no hostname": client(testToken, ""),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.Register(ctx, &protobuf.RegisterRequest{Hostname: ptr.Take("r1"), Port: ptr.Take(int32(1))})
			wantCode(t, err, codes.Unauthenticated)
			_, err = c.Notify(ctx, &protobuf.Notification{Data: &protobuf.Notification_StreamStart{
				StreamStart: &protobuf.StreamStartNotification{Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(1))}},
			}})
			wantCode(t, err, codes.Unauthenticated)
		})
	}
}

func TestRegisterIsBoundToTheAuthenticatedHostname(t *testing.T) {
	ctrl := gomock.NewController(t)
	runnerDao := mock_dao.NewMockRunnerDao(ctrl)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	m := newTestManager(t, dao.DaoWrapper{RunnerDao: runnerDao, StreamsDao: streamsDao})
	client := serveManager(t, m)(testToken, "r1")
	ctx := context.Background()

	// Claiming another runner's hostname would re-point its jobs and clear them.
	_, err := client.Register(ctx, &protobuf.RegisterRequest{Hostname: ptr.Take("victim"), Port: ptr.Take(int32(50057))})
	wantCode(t, err, codes.PermissionDenied)

	_, err = client.Register(ctx, &protobuf.RegisterRequest{Hostname: ptr.Take("r1"), Port: ptr.Take(int32(0))})
	wantCode(t, err, codes.InvalidArgument)

	runnerDao.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&model.Runner{})).DoAndReturn(
		func(_ context.Context, r *model.Runner) error {
			if r.Hostname != "r1" || r.Port != 50057 {
				t.Errorf("unexpected runner %+v", r)
			}
			return nil
		})
	streamsDao.EXPECT().ClearRunnerJobsByHostname("r1").Return(nil)
	if _, err := client.Register(ctx, &protobuf.RegisterRequest{Hostname: ptr.Take("r1"), Port: ptr.Take(int32(50057))}); err != nil {
		t.Fatalf("own hostname: %v", err)
	}
}

func TestHeartbeatIsBoundToTheAuthenticatedHostname(t *testing.T) {
	ctrl := gomock.NewController(t)
	runnerDao := mock_dao.NewMockRunnerDao(ctrl)
	m := newTestManager(t, dao.DaoWrapper{RunnerDao: runnerDao})
	client := serveManager(t, m)(testToken, "r1")

	_, err := client.Notify(context.Background(), &protobuf.Notification{Data: &protobuf.Notification_Heartbeat{
		Heartbeat: &protobuf.HeartbeatNotification{Hostname: ptr.Take("victim")},
	}})
	wantCode(t, err, codes.PermissionDenied)

	runnerDao.EXPECT().Get(gomock.Any(), "r1").Return(model.Runner{Hostname: "r1"}, nil)
	runnerDao.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	_, err = client.Notify(context.Background(), &protobuf.Notification{Data: &protobuf.Notification_Heartbeat{
		Heartbeat: &protobuf.HeartbeatNotification{Hostname: ptr.Take("r1")},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func streamStart(id uint64, url *string) *protobuf.Notification {
	return &protobuf.Notification{Data: &protobuf.Notification_StreamStart{
		StreamStart: &protobuf.StreamStartNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(id)},
			StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED),
			Url:           url,
		},
	}}
}

// A runner may only report on streams gocast gave to it. The playlist URL is what every
// viewer's player loads, so this is the check that stops a stream hijack.
func TestStreamNotificationsRequireTheOwningRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	m := newTestManager(t, dao.DaoWrapper{StreamsDao: streamsDao})
	client := serveManager(t, m)(testToken, "attacker")

	streamsDao.EXPECT().GetRunnerJobsForStream(uint(7)).Return([]model.StreamRunnerJob{
		{StreamID: 7, Version: model.COMB, RunnerHostname: "r1", JobID: "j"},
	}, nil).Times(2)

	_, err := client.Notify(context.Background(), streamStart(7, ptr.Take("https://evil.example/x.m3u8")))
	wantCode(t, err, codes.PermissionDenied)

	_, err = client.Notify(context.Background(), &protobuf.Notification{Data: &protobuf.Notification_StreamEnd{
		StreamEnd: &protobuf.StreamEndNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(7))},
			StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED),
		},
	}})
	wantCode(t, err, codes.PermissionDenied)
}

// A StreamStart without a url used to dereference a nil pointer and, with no recovery in
// the server, take the whole process down.
func TestStreamStartWithoutURLIsAnErrorNotAPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	m := newTestManager(t, dao.DaoWrapper{StreamsDao: streamsDao})
	client := serveManager(t, m)(testToken, "r1")

	streamsDao.EXPECT().GetRunnerJobsForStream(uint(7)).Return([]model.StreamRunnerJob{
		{StreamID: 7, Version: model.COMB, RunnerHostname: "r1", JobID: "j"},
	}, nil)

	_, err := client.Notify(context.Background(), streamStart(7, nil))
	wantCode(t, err, codes.InvalidArgument)

	// The server is still alive.
	_, err = client.Notify(context.Background(), &protobuf.Notification{})
	wantCode(t, err, codes.Unimplemented)
}

// VodReady arrives after the job was cleared, so it is bound to a registered runner. A
// missing stream_version used to panic here too.
func TestVodReadyRequiresRegisteredRunnerAndSurvivesMissingVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	runnerDao := mock_dao.NewMockRunnerDao(ctrl)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	m := newTestManager(t, dao.DaoWrapper{RunnerDao: runnerDao, StreamsDao: streamsDao})
	client := serveManager(t, m)(testToken, "r1")
	vod := &protobuf.Notification{Data: &protobuf.Notification_VodReady{
		VodReady: &protobuf.VODReadyNotification{Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(7))}, Url: ptr.Take("https://vod/x.m3u8")},
	}}

	runnerDao.EXPECT().Get(gomock.Any(), "r1").Return(model.Runner{}, status.Error(codes.NotFound, "no"))
	_, err := client.Notify(context.Background(), vod)
	wantCode(t, err, codes.PermissionDenied)

	// Missing stream_version: used to be a nil dereference, now a clean rejection.
	runnerDao.EXPECT().Get(gomock.Any(), "r1").Return(model.Runner{Hostname: "r1"}, nil).Times(2)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "7").Return(model.Stream{}, nil).Times(2)
	_, err = client.Notify(context.Background(), vod)
	wantCode(t, err, codes.InvalidArgument)

	vod.GetVodReady().StreamVersion = ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED)
	streamsDao.EXPECT().SaveStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error {
		if s.PlaylistUrl != "https://vod/x.m3u8" || !s.Recording {
			t.Errorf("unexpected stream %+v", s)
		}
		return nil
	})
	if _, err := client.Notify(context.Background(), vod); err != nil {
		t.Fatal(err)
	}
}

// The manager has to present the token to runners, or an authenticated runner would
// reject every job it is given.
func TestManagerAuthenticatesToRunners(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeRunnerServer{}
	srv := grpc.NewServer(grpc.UnaryInterceptor(auth.UnaryServerInterceptor(testToken, false)))
	protobuf.RegisterRunnerServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	host, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)

	ctrl := gomock.NewController(t)
	runnerDao := mock_dao.NewMockRunnerDao(ctrl)
	runnerDao.EXPECT().ReserveRunner(gomock.Any()).Return(model.Runner{Hostname: host, Port: uint32(port)}, nil).Times(2)

	_, client, conn, err := newTestManager(t, dao.DaoWrapper{RunnerDao: runnerDao}).getClient(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := client.RequestStreamEnd(context.Background(), &protobuf.StreamEndRequest{JobId: ptr.Take("j")}); err != nil {
		t.Fatalf("manager with token: %v", err)
	}
	if !fake.called {
		t.Fatal("runner was not reached")
	}

	wrong := New(dao.DaoWrapper{RunnerDao: runnerDao}, WithRunnerToken("other"))
	_, client, conn, err = wrong.getClient(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_, err = client.RequestStreamEnd(context.Background(), &protobuf.StreamEndRequest{JobId: ptr.Take("j")})
	wantCode(t, err, codes.Unauthenticated)
}
