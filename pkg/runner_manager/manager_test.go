package runner_manager

import (
	"context"
	"net"
	"runtime"
	"strconv"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"

	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestManagerOptions(t *testing.T) {
	m := Manager{}
	m.applyOpts([]Option{WithListenAddr(":1")})
	if m.listenAddr != ":1" {
		t.Errorf("m.listenAddr want: %v have: %v", ":1", m.listenAddr)
	}
}

// TestEndStream_NoJobs verifies that ending a stream with no tracked runner jobs
// (e.g. because it ran on a legacy worker, or never started) still sets the stream
// not-live and doesn't error.
func TestEndStream_NoJobs(t *testing.T) {
	ctrl := gomock.NewController(t)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetRunnerJobsForStream(uint(1)).Return([]model.StreamRunnerJob{}, nil)
	streamsDao.EXPECT().SetStreamNotLiveById(uint(1)).Return(nil)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "1").Return(model.Stream{}, nil)
	streamsDao.EXPECT().GetLiveStreamsInLectureHall(uint(0)).Return(nil, nil)

	lectureHallsDao := mock_dao.NewMockLectureHallsDao(ctrl)
	lectureHallsDao.EXPECT().GetLectureHallByID(uint(0)).Return(model.LectureHall{}, nil)

	m := New(dao.DaoWrapper{StreamsDao: streamsDao, LectureHallsDao: lectureHallsDao})

	if err := m.EndStream(context.Background(), 1, false); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// fakeRunnerServer is a minimal RunnerService implementation used to assert that
// EndStream dials the correct runner and forwards the right job id / discard flag.
type fakeRunnerServer struct {
	protobuf.UnimplementedRunnerServiceServer
	receivedJobID      string
	receivedDiscardVod bool
	called             bool
}

func (f *fakeRunnerServer) RequestStreamEnd(_ context.Context, req *protobuf.StreamEndRequest) (*protobuf.StreamEndResponse, error) {
	f.called = true
	f.receivedJobID = req.GetJobId()
	f.receivedDiscardVod = req.GetDiscardVod()
	return &protobuf.StreamEndResponse{}, nil
}

func TestEndStream_CallsRequestStreamEndOnCorrectRunner(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer func() { _ = lis.Close() }()

	fake := &fakeRunnerServer{}
	grpcServer := grpc.NewServer()
	protobuf.RegisterRunnerServiceServer(grpcServer, fake)
	go func() { _ = grpcServer.Serve(lis) }()
	defer grpcServer.Stop()

	host, portStr, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	runner := model.Runner{Hostname: host, Port: uint32(port)}

	ctrl := gomock.NewController(t)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetRunnerJobsForStream(uint(42)).Return([]model.StreamRunnerJob{
		{StreamID: 42, Version: model.COMB, RunnerHostname: host, JobID: "job-123"},
	}, nil)
	streamsDao.EXPECT().ClearRunnerJobForStream(uint(42), model.COMB).Return(nil)
	streamsDao.EXPECT().SetStreamNotLiveById(uint(42)).Return(nil)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "42").Return(model.Stream{}, nil)
	streamsDao.EXPECT().GetLiveStreamsInLectureHall(uint(0)).Return(nil, nil)

	runnerDao := mock_dao.NewMockRunnerDao(ctrl)
	runnerDao.EXPECT().Get(gomock.Any(), host).Return(runner, nil)

	lectureHallsDao := mock_dao.NewMockLectureHallsDao(ctrl)
	lectureHallsDao.EXPECT().GetLectureHallByID(uint(0)).Return(model.LectureHall{}, nil)

	m := New(dao.DaoWrapper{StreamsDao: streamsDao, RunnerDao: runnerDao, LectureHallsDao: lectureHallsDao})

	if err := m.EndStream(context.Background(), 42, true); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !fake.called {
		t.Fatal("expected RequestStreamEnd to be called on the runner")
	}
	if fake.receivedJobID != "job-123" {
		t.Errorf("job id want: %v have: %v", "job-123", fake.receivedJobID)
	}
	if !fake.receivedDiscardVod {
		t.Errorf("discard vod want: %v have: %v", true, fake.receivedDiscardVod)
	}
}

// TestGetClient_DoesNotLeakGoroutines guards the fix for the leak that pushed the
// production process from ~75 to ~950 goroutines: every un-closed grpc.ClientConn keeps
// its resolver and balancer goroutines alive for the lifetime of the process. getClient
// hands the connection back so callers can close it; if it ever stops doing so, or a
// caller stops closing, this grows without bound.
func TestGetClient_DoesNotLeakGoroutines(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer func() { _ = lis.Close() }()

	grpcServer := grpc.NewServer()
	protobuf.RegisterRunnerServiceServer(grpcServer, &fakeRunnerServer{})
	go func() { _ = grpcServer.Serve(lis) }()
	defer grpcServer.Stop()

	host, portStr, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	ctrl := gomock.NewController(t)
	runnerDao := mock_dao.NewMockRunnerDao(ctrl)
	runnerDao.EXPECT().
		ReserveRunner(gomock.Any()).
		Return(model.Runner{Hostname: host, Port: uint32(port)}, nil).
		AnyTimes()

	m := New(dao.DaoWrapper{RunnerDao: runnerDao})

	// Warm up, so that one-off runtime goroutines aren't counted as growth.
	_, _, conn, err := m.getClient(context.Background())
	if err != nil {
		t.Fatalf("getClient: %v", err)
	}
	if _, err := protobuf.NewRunnerServiceClient(conn).RequestStreamEnd(context.Background(), &protobuf.StreamEndRequest{}); err != nil {
		t.Fatalf("RequestStreamEnd: %v", err)
	}
	_ = conn.Close()

	baseline := settledGoroutines()

	const iterations = 20
	for i := 0; i < iterations; i++ {
		_, client, conn, err := m.getClient(context.Background())
		if err != nil {
			t.Fatalf("getClient (iteration %d): %v", i, err)
		}
		if _, err := client.RequestStreamEnd(context.Background(), &protobuf.StreamEndRequest{}); err != nil {
			t.Fatalf("RequestStreamEnd (iteration %d): %v", i, err)
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("close (iteration %d): %v", i, err)
		}
	}

	// Each leaked connection costs at least three goroutines, so anything close to
	// iterations*3 is a regression. Allow a little slack for server-side teardown.
	if growth := settledGoroutines() - baseline; growth > iterations {
		t.Errorf("goroutine growth over %d connections want: <=%d have: %d", iterations, iterations, growth)
	}
}

// settledGoroutines gives grpc teardown a moment to finish before counting.
func settledGoroutines() int {
	previous := -1
	for i := 0; i < 50; i++ {
		runtime.GC()
		current := runtime.NumGoroutine()
		if current == previous {
			return current
		}
		previous = current
		time.Sleep(20 * time.Millisecond)
	}
	return runtime.NumGoroutine()
}
