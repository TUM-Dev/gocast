package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/getsentry/sentry-go"
	"github.com/golang-jwt/jwt"
	log "github.com/sirupsen/logrus"
	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ protobuf.FromRunnerServer = nil

type GrpcRunnerServer struct {
	protobuf.UnimplementedFromRunnerServer

	dao.DaoWrapper
}

func (g GrpcRunnerServer) Register(ctx context.Context, request *protobuf.RegisterRequest) (*protobuf.RegisterResponse, error) {
	// Parse and validate worker token
	token, err := jwt.ParseWithClaims(request.Token, &JWTOrganizationClaims{}, func(token *jwt.Token) (interface{}, error) {
		key := tools.Cfg.GetJWTKey().Public()
		return key, nil
	})
	if err != nil || !token.Valid {
		return nil, status.Error(codes.Unauthenticated, "Invalid token")
	}
	claims, ok := token.Claims.(*JWTOrganizationClaims)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Invalid token claims")
	}

	organizationID, err := strconv.ParseUint(claims.OrganizationID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid OrganizationID: %v", err)
	}

	runner := model.Runner{
		Hostname:       request.Hostname,
		Port:           int(request.Port),
		LastSeen:       sql.NullTime{Valid: true, Time: time.Now()},
		Status:         "Alive",
		Workload:       0,
		OrganizationID: uint(organizationID),
	}

	err = g.RunnerDao.Create(ctx, &runner)
	if err != nil {
		return nil, err
	}
	return &protobuf.RegisterResponse{}, nil
}

func (g GrpcRunnerServer) Heartbeat(ctx context.Context, request *protobuf.HeartbeatRequest) (*protobuf.HeartbeatResponse, error) {
	runner := model.Runner{
		Hostname: request.Hostname,
		Port:     int(request.Port),
	}

	r, err := g.RunnerDao.Get(ctx, runner.Hostname)
	if err != nil {
		log.WithError(err).Error("Failed to get runner")
		return &protobuf.HeartbeatResponse{Ok: false}, err
	}

	newStats := model.Runner{
		Hostname: request.Hostname,
		Port:     int(request.Port),
		LastSeen: sql.NullTime{Valid: true, Time: time.Now()},
		Status:   "Alive",
		Workload: uint(request.Workload),
		CPU:      request.CPU,
		Memory:   request.Memory,
		Disk:     request.Disk,
		Uptime:   request.Uptime,
		Version:  request.Version,
	}
	ctx = context.WithValue(ctx, "newStats", newStats)
	log.Info("Updating runner stats ", "runner", r)
	p, err := r.UpdateStats(dao.DB, ctx)
	return &protobuf.HeartbeatResponse{Ok: p}, err
}

func StreamRequest(ctx context.Context, dao dao.DaoWrapper, stream model.Stream, course model.Course, source string, runners []model.Runner, version string, end time.Time) {
	if source == "" {
		return
	}
	server, err := dao.IngestServerDao.GetBestIngestServer(course.OrganizationID)
	if err != nil {
		logger.Error("can't find ingest server", "err", err)
		return
	}

	var slot model.StreamName
	if version == "COMB" { // try to find a transcoding slot for comb view:
		slot, err = dao.IngestServerDao.GetTranscodedStreamSlot(server.ID)
	}
	if version != "COMB" || err != nil {
		slot, err = dao.IngestServerDao.GetStreamSlot(server.ID)
		if err != nil {
			logger.Error("No free stream slot", "err", err)
			return
		}
	}
	src := "rtsp://" + source
	slot.StreamID = stream.ID
	dao.IngestServerDao.SaveSlot(slot)
	req := protobuf.StreamRequest{
		Stream:  uint64(stream.ID),
		Course:  uint64(course.ID),
		Version: version,
		End:     timestamppb.New(end),
		Source:  src,
	}
	// get runner with least workload for given job
	runner, err := getRunnerWithLeastWorkloadForJob(runners, "stream")
	if err != nil {
		logger.Error("No runners available", "err", err)
		return
	}
	err = dao.StreamsDao.SetStreamRequested(stream)
	if err != nil {
		logger.Error("Can't set stream requested", "err", err)
		return
	}
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", runner.Hostname, runner.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Can't dial runner", "err", err)
		return
	}
	client := protobuf.NewToRunnerClient(conn)
	resp, err := client.RequestStream(context.Background(), &req)
	if err != nil {
		logger.Error("Can't request stream", "err", err)
		return
	}
	logger.Info("Stream requested", "jobID", resp.Job)
	if err = conn.Close(); err != nil {
		logger.Error("Can't close connection", "err", err)
	}
}

func getRunnerWithLeastWorkloadForJob(runner []model.Runner, Job string) (model.Runner, error) {
	if len(runner) == 0 {
		return model.Runner{}, errors.New("runner array is empty")
	}
	chosen := runner[0]
	for _, r := range runner {
		if r.Workload < chosen.Workload {
			chosen = r
		}
	}
	return chosen, nil
}

// RequestSelfStream is called by the runner when a stream is supposed to be started by obs or other third party software
// returns an error if anything goes wrong OR the stream may not be published
func (g GrpcRunnerServer) RequestSelfStream(ctx context.Context, request *protobuf.SelfStreamRequest) (*protobuf.SelfStreamResponse, error) {
	// TODO Test me/Improve me
	if request.StreamKey == "" {
		return nil, errors.New("stream key is empty")
	}
	stream, err := g.StreamsDao.GetStreamByKey(ctx, request.StreamKey)
	if err != nil {
		return nil, err
	}
	course, err := g.CoursesDao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return nil, err
	}
	if !(time.Now().After(stream.Start.Add(time.Minute*-30)) && time.Now().Before(stream.End.Add(time.Minute*30))) {
		log.WithFields(log.Fields{"streamId": stream.ID}).Warn("Stream rejected, time out of bounds")
		return nil, errors.New("stream rejected")
	}
	ingestServer, err := g.IngestServerDao.GetBestIngestServer(course.OrganizationID)
	if err != nil {
		return nil, err
	}
	slot, err := g.IngestServerDao.GetStreamSlot(ingestServer.ID)
	if err != nil {
		return nil, err
	}
	slot.StreamID = stream.ID
	g.IngestServerDao.SaveSlot(slot)

	return &protobuf.SelfStreamResponse{
		Stream:       uint64(stream.ID),
		Course:       uint64(course.ID),
		CourseYear:   uint64(course.Year),
		StreamStart:  timestamppb.New(stream.Start),
		StreamEnd:    timestamppb.New(stream.End),
		UploadVoD:    course.VODEnabled,
		IngestServer: ingestServer.Url,
		StreamName:   stream.StreamName,
		OutURL:       ingestServer.OutUrl,
	}, nil
}

func (g GrpcRunnerServer) NotifyStreamStarted(ctx context.Context, request *protobuf.StreamStarted) (*protobuf.Status, error) {
	mutex.Lock()
	defer mutex.Unlock()
	runner, err := g.RunnerDao.Get(ctx, request.Hostname)
	if err != nil {
		logger.Error("Failed to get runner", slog.String("err", err.Error()))
		return nil, err
	}
	stream, err := g.StreamsDao.GetStreamByID(ctx, fmt.Sprintf("%d", request.StreamID))
	if err != nil {
		logger.Error("Failed to get stream", slog.String("err", err.Error()))
		return nil, err
	}
	course, err := g.CoursesDao.GetCourseById(ctx, (uint)(request.CourseID))
	if err != nil {
		logger.Error("Failed to get course", slog.String("err", err.Error()))
		return nil, err
	}
	go func() {
		err := handleLightOnSwitch(stream, g.DaoWrapper)
		if err != nil {
			logger.Error("Can't handle light on switch", "err", err)
		}
		err = handleCameraPositionSwitch(stream, g.DaoWrapper)
		if err != nil {
			logger.Error("Can't handle camera position switch", "err", err)
		}
		err = g.DaoWrapper.DeleteSilences(fmt.Sprintf("%d", stream.ID))
		if err != nil {
			logger.Error("Can't delete silences", "err", err)
		}
	}()
	go func() {
		stream.LiveNow = true
		stream.Private = course.LivePrivate

		err := g.StreamsDao.SaveStream(&stream)
		if err != nil {
			logger.Error("Failed to save stream", slog.String("err", err.Error()))
		}
		err = g.StreamsDao.SetStreamLiveNowTimestampById(uint(request.StreamID), time.Now())
		if err != nil {
			logger.Error("Can't set StreamLiveNowTimestamp", "err", err)
		}

		time.Sleep(time.Second * 5)
		if !isHLSUrlOk(request.HLSUrl) {
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetExtra("URL", request.HLSUrl)
				scope.SetExtra("StreamID", request.StreamID)
				scope.SetExtra("LectureHall", stream.LectureHallID)
				scope.SetExtra("Runner", runner.Hostname)
				scope.SetExtra("Version", request.SourceType)
				sentry.CaptureException(errors.New("DVR URL 404s"))
			})
			request.HLSUrl = strings.ReplaceAll(request.HLSUrl, "?dvr", "")
		}

		switch request.Version {
		case "CAM":
			g.StreamsDao.SaveCAMURL(&stream, request.HLSUrl)
		case "PRES":
			g.StreamsDao.SavePRESURL(&stream, request.HLSUrl)
		default:
			g.StreamsDao.SaveCOMBURL(&stream, request.HLSUrl)
		}

		NotifyViewersLiveState(stream.Model.ID, true)
		NotifyLiveUpdateCourseWentLive(stream.Model.ID)
	}()
	return &protobuf.Status{Ok: true}, nil
}

func isHLSUrlOk(url string) bool {
	r, err := http.Get(url)
	if err != nil {
		return false
	}
	all, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	re, _ := regexp.Compile(`chunklist.*\.m3u8`)
	x := re.Find(all)
	if x == nil {
		return false
	}
	y := strings.ReplaceAll(r.Request.URL.String(), "playlist.m3u8", string(x))
	get, err := http.Get(y)
	if err != nil {
		return false
	}
	if get.StatusCode == http.StatusNotFound {
		return false
	}
	return true
}

func NotifyRunners(dao dao.DaoWrapper) func() {
	return func() {
		logger.Info("Notifying runners")

		organizationsStreams := dao.StreamsDao.GetDueStreamsForWorkers()
		for organizationID, streams := range organizationsStreams {
			runners, err := dao.RunnerDao.GetAll(context.Background(), organizationID)
			if err != nil {
				logger.Error("Can't get runners for organization", slog.String("err", err.Error()))
				continue
			}
			if len(runners) == 0 {
				logger.Error("No runners available for organization")
				continue
			}

			for i := range streams {
				err = dao.StreamsDao.SaveEndedState(streams[i].ID, false)
				if err != nil {
					logger.Warn("Can't save ended state", slog.String("err", err.Error()))
					sentry.CaptureException(err)
					continue
				}
				courseForStream, err := dao.CoursesDao.GetCourseById(context.Background(), streams[i].CourseID)
				if err != nil {
					logger.Warn("Can't get course for stream", slog.String("err", err.Error()))
					sentry.CaptureException(err)
					continue
				}
				lectureHallForStream, err := dao.LectureHallsDao.GetLectureHallByID(streams[i].LectureHallID)
				if err != nil {
					logger.Warn("Can't get lecture hall for stream", slog.String("err", err.Error()))
					sentry.CaptureException(err)
					continue
				}

				switch courseForStream.GetSourceModeForLectureHall(streams[i].LectureHallID) {
				case 1: // presentation
					StreamRequest(context.Background(), dao, streams[i], courseForStream, lectureHallForStream.PresIP, runners, "PRES", streams[i].End)
					break
				case 2: // camera
					StreamRequest(context.Background(), dao, streams[i], courseForStream, lectureHallForStream.CamIP, runners, "CAM", streams[i].End)
					break
				default: // combined
					StreamRequest(context.Background(), dao, streams[i], courseForStream, lectureHallForStream.PresIP, runners, "PRES", streams[i].End)
					StreamRequest(context.Background(), dao, streams[i], courseForStream, lectureHallForStream.CamIP, runners, "CAM", streams[i].End)
					StreamRequest(context.Background(), dao, streams[i], courseForStream, lectureHallForStream.CombIP, runners, "COMB", streams[i].End)
					break
				}
			}
		}
	}
}

func (g GrpcRunnerServer) mustEmbedUnimplementedFromRunnerServer() {
	// TODO implement me
	panic("implement me")
}

func StartGrpcRunnerServer() {
	lis, err := net.Listen("tcp", ":50056")
	if err != nil {
		log.WithError(err).Error("Failed to init grpc server")
		return
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute * 5,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	protobuf.RegisterFromRunnerServer(grpcServer, &GrpcRunnerServer{DaoWrapper: dao.NewDaoWrapper()})
	reflection.Register(grpcServer)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.WithError(err).Errorf("Can't serve grpc")
		}
	}()
}
