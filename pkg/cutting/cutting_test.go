package cutting

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"

	"github.com/tum-dev/gocast/runner/protobuf"

	apiv2 "github.com/TUM-Dev/gocast/apiv2/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
)

func TestWithoutManager(t *testing.T) {
	c := Cutting{}
	for name, call := range map[string]func() error{
		"RequestWaveform": func() error { return c.RequestWaveform(1, []apiv2.CutSource{{Version: model.COMB}}) },
		"Cut":             func() error { return c.Cut(1, 2, apiv2.CutSource{Version: model.COMB}, nil) },
		"ScanSilences":    func() error { return c.ScanSilences(1) },
	} {
		if err := call(); err == nil || err.Error() != "no runner configured" {
			t.Errorf("%s: err = %v, want no runner configured", name, err)
		}
	}
}

// fakeRunner records the waveform and cut requests it receives.
type fakeRunner struct {
	protobuf.UnimplementedRunnerServiceServer
	waveform *protobuf.WaveformRequest
	cut      *protobuf.CutVODRequest
}

func (f *fakeRunner) RequestWaveform(_ context.Context, req *protobuf.WaveformRequest) (*protobuf.WaveformResponse, error) {
	f.waveform = req
	return &protobuf.WaveformResponse{}, nil
}

func (f *fakeRunner) CutVOD(_ context.Context, req *protobuf.CutVODRequest) (*protobuf.CutVODResponse, error) {
	f.cut = req
	return &protobuf.CutVODResponse{}, nil
}

// withRunner serves fake on a local port and returns a Cutting whose manager reserves it.
func withRunner(t *testing.T, fake *fakeRunner) Cutting {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	protobuf.RegisterRunnerServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	host, portStr, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	runnerDao := mock_dao.NewMockRunnerDao(gomock.NewController(t))
	runnerDao.EXPECT().ReserveRunner(gomock.Any()).Return(model.Runner{Hostname: host, Port: uint32(port)}, nil)
	return Cutting{Manager: runner_manager.New(dao.DaoWrapper{RunnerDao: runnerDao})}
}

func TestRequestWaveform(t *testing.T) {
	fake := &fakeRunner{}
	c := withRunner(t, fake)

	err := c.RequestWaveform(5, []apiv2.CutSource{
		{Version: model.COMB, PlaylistURL: "https://edge/comb.m3u8"},
		{Version: model.PRES, PlaylistURL: "https://edge/pres.m3u8"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if fake.waveform.GetStreamId() != 5 {
		t.Errorf("stream id = %d, want 5", fake.waveform.GetStreamId())
	}
	want := []struct {
		version protobuf.StreamVersion
		url     string
	}{
		{protobuf.StreamVersion_STREAM_VERSION_COMBINED, "https://edge/comb.m3u8"},
		{protobuf.StreamVersion_STREAM_VERSION_PRESENTATION, "https://edge/pres.m3u8"},
	}
	got := fake.waveform.GetSources()
	if len(got) != len(want) {
		t.Fatalf("sources = %v, want %d", got, len(want))
	}
	for i, w := range want {
		if got[i].GetVersion() != w.version || got[i].GetPlaylistUrl() != w.url {
			t.Errorf("source %d = %v, want %v %s", i, got[i], w.version, w.url)
		}
	}
}

func TestCut(t *testing.T) {
	fake := &fakeRunner{}
	c := withRunner(t, fake)

	src := apiv2.CutSource{Version: model.CAM, PlaylistURL: "https://edge/cam.m3u8"}
	if err := c.Cut(3, 8, src, []model.CutRange{{Start: 0, End: 12.5}, {Start: 90, End: 100}}); err != nil {
		t.Fatal(err)
	}

	got := fake.cut
	if got.GetStreamId() != 3 || got.GetTargetStreamId() != 8 {
		t.Errorf("stream %d to %d, want 3 to 8", got.GetStreamId(), got.GetTargetStreamId())
	}
	if got.GetVersion() != protobuf.StreamVersion_STREAM_VERSION_CAMERA || got.GetPlaylistUrl() != src.PlaylistURL {
		t.Errorf("version %v at %q, want camera at %q", got.GetVersion(), got.GetPlaylistUrl(), src.PlaylistURL)
	}
	d := got.GetDiscards()
	if len(d) != 2 || d[0].GetStart() != 0 || d[0].GetEnd() != 12.5 || d[1].GetStart() != 90 || d[1].GetEnd() != 100 {
		t.Errorf("discards = %v, want [0, 12.5] [90, 100]", d)
	}
}

func TestScanSilences(t *testing.T) {
	notFound := errors.New("not found")
	streams := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	streams.EXPECT().GetStreamByID(gomock.Any(), "4").Return(model.Stream{}, notFound)
	c := Cutting{Manager: runner_manager.New(dao.DaoWrapper{StreamsDao: streams})}

	if err := c.ScanSilences(4); !errors.Is(err, notFound) {
		t.Errorf("err = %v, want %v", err, notFound)
	}
}
