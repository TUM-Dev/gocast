package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var _ protobuf.FromRunnerServer = nil

type GrpcRunnerServer struct {
	protobuf.UnimplementedFromRunnerServer

	dao.DaoWrapper
}

//

func (g GrpcRunnerServer) Register(ctx context.Context, request *protobuf.RegisterRequest) (*protobuf.RegisterResponse, error) {
	runner := model.Runner{
		Hostname: request.Hostname,
		Port:     int(request.Port),
		LastSeen: time.Now(),
		Status:   "Alive",
		Workload: 0,
	}
	err := g.RunnerDao.Create(ctx, &runner)
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
		LastSeen: time.Now(),
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

func StreamRequest(ctx context.Context, dao dao.DaoWrapper, runner model.Runner) {
	streamID := fmt.Sprintf("%f", ctx.Value("stream"))
	stream, err := dao.StreamsDao.GetStreamByID(ctx, streamID)
	if err != nil {
		logger.Error("Can't get stream", "err", err)
		return
	}
	course, err := dao.CoursesDao.GetCourseById(ctx, uint(ctx.Value("course").(float64)))
	if err != nil {
		logger.Error("Can't get course", "err", err)
		return
	}
	source := fmt.Sprintf("%v", ctx.Value("source"))
	version := fmt.Sprintf("%v", ctx.Value("version"))
	actionID := fmt.Sprintf("%v", ctx.Value("actionID"))
	stringEnd := fmt.Sprintf("%v", ctx.Value("end"))
	end, err := time.Parse("2006-01-02T15:04:05+02:00", stringEnd)
	if err != nil {
		logger.Error("Can't parse end", "err", err)
		return
	}
	if source == "" {
		logger.Error("No source", "source", source)
		return
	}
	server, err := dao.IngestServerDao.GetBestIngestServer()
	if err != nil {
		logger.Error("can't find ingest server", "err", err)
		return
	}

	var slot model.StreamName
	if version == "COMB" { //try to find a transcoding slot for comb view:
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
		ActionID: actionID,
		Stream:   uint64(stream.ID),
		Course:   uint64(course.ID),
		Version:  version,
		End:      timestamppb.New(end),
		Source:   src,
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
	logger.Info("Stream requested", "ActionID", resp.ActionID)
	if err = conn.Close(); err != nil {
		logger.Error("Can't close connection", "err", err)
	}

	return
}
func TranscodingRequest(ctx context.Context, dao dao.DaoWrapper, runner model.Runner) {
	stream, err := dao.StreamsDao.GetStreamByID(ctx, ctx.Value("stream").(string))
	if err != nil {
		logger.Error("Can't get stream", "err", err)
		return
	}
	course, err := dao.CoursesDao.GetCourseById(ctx, ctx.Value("course").(uint))
	if err != nil {
		logger.Error("Can't get course", "err", err)
		return
	}
	source := ctx.Value("source").(string)
	version := ctx.Value("version").(string)
	actionID := ctx.Value("actionID").(string)

	if source == "" {
		return
	}

	//gather all data into one part url

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", runner.Hostname, runner.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Can't dial runner", "err", err)
		return
	}
	client := protobuf.NewToRunnerClient(conn)
	resp, err := client.RequestTranscoding(context.Background(), &protobuf.TranscodingRequest{
		ActionID:   actionID,
		DataURL:    "",
		RunnerID:   runner.Hostname,
		StreamName: stream.StreamName,
		CourseName: course.Name,
		SourceType: version,
	})
	if err != nil {
		logger.Error("Can't request transcode", "err", err)
		return
	}
	logger.Info("Transcode requested", "actionID", resp.ActionID)
	if err = conn.Close(); err != nil {
		logger.Error("Can't close connection", "err", err)
	}

}

func getRunnerWithLeastWorkloadForJob(runner []model.Runner, Job string) (model.Runner, error) {
	if len(runner) == 0 {
		return model.Runner{}, errors.New("runner array is empty")
	}
	chosen := runner[0]
	switch Job {

	}
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
	//TODO Test me/Improve me
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
	ingestServer, err := g.IngestServerDao.GetBestIngestServer()
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

func (g GrpcRunnerServer) NotifyStreamEnd(ctx context.Context, request *protobuf.StreamEndRequest) (*protobuf.StreamEndResponse, error) {
	//TODO Test me
	stream, err := g.StreamsDao.GetStreamByID(ctx, fmt.Sprintf("%v", request.ActionID))
	if err != nil {
		return nil, err
	}
	err = g.StreamsDao.SaveEndedState(stream.ID, true)
	if err != nil {
		return nil, err
	}
	return &protobuf.StreamEndResponse{}, nil

}

func (g GrpcRunnerServer) NotifyStreamStarted(ctx context.Context, request *protobuf.StreamStarted) (*protobuf.Status, error) {
	mutex.Lock()
	defer mutex.Unlock()
	runner, err := g.RunnerDao.Get(ctx, request.Hostname)
	if err != nil {
		logger.Error("Failed to get runner", err)
		return nil, err
	}
	stream, err := g.StreamsDao.GetStreamByID(ctx, fmt.Sprintf("%d", request.StreamID))
	if err != nil {
		logger.Error("Failed to get stream", err)
		return nil, err
	}
	course, err := g.CoursesDao.GetCourseById(ctx, (uint)(request.CourseID))
	if err != nil {
		logger.Error("Failed to get course", err)
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
			logger.Error("Failed to save stream", err)
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

func (g GrpcRunnerServer) NotifyVoDUploadFinished(ctx context.Context, request *protobuf.VoDUploadFinished) (*protobuf.Status, error) {

	panic("implement!")
}

func (g GrpcRunnerServer) NotifyActionFinished(ctx context.Context, request *protobuf.ActionFinished) (*protobuf.Status, error) {
	_, err := g.RunnerDao.Get(ctx, request.RunnerID)

	status := &protobuf.Status{Ok: false}
	switch request.Type {
	case "Upload":
		status, err = SetUploadFinished(ctx, request)
		if err != nil {
			return nil, err
		}
	case "Transcode":
		status, err = SetTranscodeFinished(ctx, request)
		if err != nil {
			return nil, err
		}
	case "Stream":
	}

	return &protobuf.Status{Ok: status.Ok}, nil

}

func SetUploadFinished(ctx context.Context, req *protobuf.ActionFinished) (*protobuf.Status, error) {
	panic("implement me")
}

func SetTranscodeFinished(ctx context.Context, req *protobuf.ActionFinished) (*protobuf.Status, error) {
	panic("implement me")
}

func NotifyForStreams(dao dao.DaoWrapper) func() {
	return func() {

		logger.Info("Notifying runners")

		streams := dao.StreamsDao.GetDueStreamsForWorkers()
		logger.Info("incoming stream count", "count", len(streams))
		for i := range streams {
			err := dao.StreamsDao.SaveEndedState(streams[i].ID, false)
			if err != nil {
				logger.Warn("Can't save ended state", err)
				sentry.CaptureException(err)
				continue
			}
			courseForStream, err := dao.CoursesDao.GetCourseById(context.Background(), streams[i].CourseID)
			if err != nil {
				logger.Warn("Can't get course for stream", err)
				sentry.CaptureException(err)
				continue
			}
			lectureHallForStream, err := dao.LectureHallsDao.GetLectureHallByID(streams[i].LectureHallID)
			if err != nil {
				logger.Warn("Can't get lecture hall for stream", err)
				sentry.CaptureException(err)
				continue
			}
			ctx := context.WithValue(context.Background(), "type", "stream")
			values := map[string]interface{}{
				"type":   "stream",
				"stream": streams[i].ID,
				"course": courseForStream.ID,
				"end":    streams[i].End,
			}
			switch courseForStream.GetSourceModeForLectureHall(streams[i].LectureHallID) {
			case 1:
				values["version"] = "PRES"
				values["source"] = lectureHallForStream.PresIP
				err = CreateJob(dao, ctx, values) //presentation
				if err != nil {
					logger.Error("Can't create job", err)
				}
				break
			case 2: //camera
				values["version"] = "CAM"
				values["source"] = lectureHallForStream.CamIP
				err = CreateJob(dao, ctx, values)
				if err != nil {
					logger.Error("Can't create job", err)
				}
				break
			default: //combined
				values["version"] = "PRES"
				values["source"] = lectureHallForStream.PresIP
				err = CreateJob(dao, ctx, values)

				if err != nil {
					logger.Error("Can't create job", err)
				}

				values["version"] = "CAM"
				values["source"] = lectureHallForStream.CamIP
				err = CreateJob(dao, ctx, values)
				if err != nil {
					logger.Error("Can't create job", err)
				}

				values["version"] = "COMB"
				values["source"] = lectureHallForStream.CombIP
				err = CreateJob(dao, ctx, values)
				if err != nil {
					logger.Error("Can't create job", err)
				}
				break
			}
		}
	}
}

func NotifyRunnerAssignments(dao dao.DaoWrapper) func() {
	return func() {
		logger.Info("Assigning runners to action")
		ctx := context.Background()

		//Running normal jobs with the idea that they are working as they should
		jobs, err := dao.JobDao.GetAllOpenJobs(ctx)
		if err != nil {
			logger.Error("Can't get jobs", err)
			return
		}
		for _, job := range jobs {
			action, err := job.GetNextAction()
			if err != nil {
				logger.Error("Can't get next action", err)
				continue
			}
			err = AssignRunnerAction(dao, action)
			if err != nil {
				logger.Error("Can't assign runner to action", err)
				continue
			}
		}
		//checking for each running action if the runner is still doing the job or if it is dead
		activeAction, err := dao.ActionDao.GetRunningActions(ctx)
		if err != nil {
			logger.Error("Can't get running actions", err)
		}
		for _, action := range activeAction {
			runner := action.GetCurrentRunner()
			if !runner.IsAlive() && action.IsCompleted() {
				action.SetToFailed()
			}
		}

		failedActions, err := dao.ActionDao.GetAllFailedActions(ctx)
		if err != nil {
			logger.Error("Can't get failed actions", err)
			return
		}
		for _, failedAction := range failedActions {
			failedAction.SetToRestarted()
			err = AssignRunnerAction(dao, &failedAction)
			if err != nil {
				logger.Error("Can't assign runner to action", err)
			}
		}
	}
}

func AssignRunnerAction(dao dao.DaoWrapper, action *model.Action) error {
	//here is where we are going to selectively get the runner for each type of action
	runners, err := dao.RunnerDao.GetAll(context.Background())
	if err != nil {
		return err
	}
	if len(runners) == 0 {
		logger.Error("No runners available")
		return err
	}
	runner, err := getRunnerWithLeastWorkloadForJob(runners, action.Type)
	action.AssignRunner(runner)
	ctx := context.Background()

	if err != nil {
		logger.Error("Can't unmarshal json", err)
		return err
	}
	values := map[string]interface{}{}
	err = json.Unmarshal([]byte(action.Values), &values)
	for key, value := range values {
		logger.Info("values", "value", value)
		ctx = context.WithValue(ctx, key, value)
	}
	ctx = context.WithValue(ctx, "actionID", fmt.Sprintf("%v", action.ID))

	switch action.Type {
	case "stream":
		StreamRequest(ctx, dao, runner)
		break
	case "transcoding":
		TranscodingRequest(ctx, dao, runner)
		break
	}
	action.SetToRunning()
	return nil
}

func CreateJob(dao dao.DaoWrapper, ctx context.Context, values map[string]interface{}) error {
	job := model.Job{
		Start:     time.Now(),
		Completed: false,
	}
	value, err := json.Marshal(values)
	if err != nil {
		return err
	}
	var actions []model.Action
	switch ctx.Value("type") {
	case "stream":
		actions = append(actions, model.Action{
			Status: 3,
			Type:   "stream",
			Values: string(value),
		}, model.Action{
			Status: 3,
			Type:   "transcode",
			Values: string(value),
		}, model.Action{
			Status: 3,
			Type:   "upload",
			Values: string(value),
		})
		job.Actions = append(job.Actions, actions...)
	}
	err = dao.CreateJob(ctx, job)
	if err != nil {
		logger.Error("couldn't create job in database", err)
		return err
	}

	return nil
}

func (g GrpcRunnerServer) mustEmbedUnimplementedFromRunnerServer() {
	//TODO implement me
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
