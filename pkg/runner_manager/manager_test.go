package runner_manager

import (
	"context"
	"net"
	"strconv"
	"testing"

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
