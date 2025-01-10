package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/getsentry/sentry-go"
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
	"strconv"
	"strings"
	"time"
)

var _ protobuf.FromRunnerServer = nil

// GrpcRunnerServer is the end note that connects all runners with TUMLive
type GrpcRunnerServer struct {
	protobuf.UnimplementedFromRunnerServer

	dao.DaoWrapper
}

/*
Register is called by the runner on start up, creating a model in the data bank and start to send heartbeats
*/
func (g GrpcRunnerServer) Register(ctx context.Context, request *protobuf.RegisterRequest) (*protobuf.RegisterResponse, error) {
	runner, err := g.RunnerDao.Get(ctx, request.Hostname)
	if runner == nil || runner.Hostname == "" {
		runner = &model.Runner{
			Hostname: request.Hostname,
			Port:     int(request.Port),
			LastSeen: time.Now(),
			Status:   "Alive",
			Workload: 0,
		}
	}
	err = g.RunnerDao.Create(ctx, runner)
	if err != nil {
		return nil, err
	}
	return &protobuf.RegisterResponse{ID: runner.Hostname}, nil
}

/*
Heartbeat is called by the runner every 30 seconds to update the stats of the runner. it contains not only the vmStats
that show how much power it has (nice to have for later runner selection, see getRunnerWithLeastWorkloadForJob) but
also saves the actions that the runner has started and running locally
*/
func (g GrpcRunnerServer) Heartbeat(ctx context.Context, request *protobuf.HeartbeatRequest) (*protobuf.HeartbeatResponse, error) {

	//get the runner model from data bank to update
	r, err := g.RunnerDao.Get(ctx, request.Hostname)
	if err != nil {
		logger.Error("Failed to get runner", "err", err)
		return &protobuf.HeartbeatResponse{Ok: false}, err
	}

	//create a new map to save the new stats
	newStat := make(map[string]interface{})
	newStat["LastSeen"] = time.Now()
	newStat["Status"] = "Alive"
	newStat["Workload"] = uint(request.Workload)
	newStat["CPU"] = request.CPU
	newStat["Memory"] = request.Memory
	newStat["Disk"] = request.Disk
	newStat["Uptime"] = request.Uptime
	newStat["Version"] = request.Version
	newStat["Actions"] = request.CurrentAction

	logger.Info("Updating runner stats ", "runner", r)

	//Update the model with the new stats
	p, err := r.UpdateStats(dao.DB, ctx, newStat)

	//return the response
	return &protobuf.HeartbeatResponse{Ok: p}, err
}

func StreamRequest(ctx context.Context, dao dao.DaoWrapper, runner model.Runner, values map[string]interface{}) {

	streamID := values["stream"].(string) //fmt.Sprintf("%f", ctx.Value("stream"))

	//get the stream and courses from the data bank
	stream, err := dao.StreamsDao.GetStreamByID(ctx, streamID)
	if err != nil {
		logger.Error("Can't get stream", "err", err)
		return
	}
	course, err := dao.CoursesDao.GetCourseById(ctx, values["course"].(uint))
	if err != nil {
		logger.Error("Can't get course", "err", err)
		return
	}
	//get all other values from the map
	source := values["source"].(string)     //fmt.Sprintf("%v", ctx.Value("source"))
	version := values["version"].(string)   //fmt.Sprintf("%v", ctx.Value("version"))
	actionID := values["actionID"].(string) //fmt.Sprintf("%v", ctx.Value("actionID"))
	stringEnd := values["end"].(string)     //fmt.Sprintf("%v", ctx.Value("end"))
	end, err := time.Parse(time.RFC3339, stringEnd)
	if err != nil {
		logger.Error("Can't parse end", "err", err)
		return
	}
	if source == "" {
		logger.Error("No source", "source", source)
		return
	}

	//TODO: Implement environment variable for ingest
	ingest := false
	if ingest {
		//this is like the old version with the ingest servers. it can be activated or not, creating an environment variable later
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
		slot.StreamID = stream.ID
		dao.IngestServerDao.SaveSlot(slot)
	}

	//setting up the values for the runner StreamRequest
	src := "rtsp://" + source
	req := protobuf.StreamRequest{
		ActionID: actionID,
		Stream:   uint64(stream.ID),
		Course:   uint64(course.ID),
		Version:  version,
		End:      timestamppb.New(end),
		Source:   src,
	}

	//creating a connection between the desired runner and TUMLive
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", runner.Hostname, runner.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Can't dial runner", "err", err)
		return
	}
	client := protobuf.NewToRunnerClient(conn)

	//sending the request to the runner
	resp, err := client.RequestStream(context.Background(), &req)
	if err != nil {
		logger.Error("Can't request stream", "err", err)
		return
	}

	//sets the stream requested
	err = dao.StreamsDao.SetStreamRequested(stream)
	if err != nil {
		logger.Error("Can't set stream requested", "err", err)
		return
	}
	logger.Info("Stream requested", "ActionID", resp.ActionID)

	//and closing the connection
	if err = conn.Close(); err != nil {
		logger.Error("Can't close connection", "err", err)
	}

	return
}
func TranscodingRequest(ctx context.Context, dao dao.DaoWrapper, runner model.Runner, values map[string]interface{}) {

	//Setting up the values from the given map
	stream, err := dao.StreamsDao.GetStreamByID(ctx, values["stream"].(string))
	if err != nil {
		logger.Error("Can't get stream", "err", err)
		return
	}
	course, err := dao.CoursesDao.GetCourseById(ctx, values["course"].(uint))
	if err != nil {
		logger.Error("Can't get course", "err", err)
		return
	}
	source := values["source"].(string)
	version := values["version"].(string)
	actionID := values["actionID"].(string)

	if source == "" {
		return
	}

	//creating a connection between the desired runner and TUMLive

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", runner.Hostname, runner.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Can't dial runner", "err", err)
		return
	}
	client := protobuf.NewToRunnerClient(conn)

	//sending the request to the runner
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

	//and closing the connection
	if err = conn.Close(); err != nil {
		logger.Error("Can't close connection", "err", err)
	}

}

func getRunnerWithLeastWorkloadForJob(runner []model.Runner, Job string) (model.Runner, error) {

	/*
		this is an unnecessary function for now that later will change depending on the workload of the runners
		TODO: make sure that at least the action count is looked at before the runner is chosen
	*/

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

	/*
		TODO Test me/Improve me
		The function is a copy from what the worker had and needs to be looked at first with the proxy
	*/
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
		logger.Warn("Stream rejected, time out of bounds", "streamID", stream.ID)
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

/*
NotifyStreamEnded is called by the runner when the stream has ended. It sets the stream to ended in the data bank
*/
func (g GrpcRunnerServer) NotifyStreamEnded(ctx context.Context, request *protobuf.StreamEnded) (*protobuf.Status, error) {

	//get the stream from the data bank
	stream, err := g.StreamsDao.GetStreamByID(ctx, fmt.Sprintf("%v", request.StreamID))
	if err != nil {
		return &protobuf.Status{Ok: false}, err
	}
	//set the stream to ended
	err = g.StreamsDao.SaveEndedState(stream.ID, true)
	if err != nil {
		return &protobuf.Status{Ok: false}, err
	}
	return &protobuf.Status{Ok: true}, nil
}

func (g GrpcRunnerServer) NotifyStreamStarted(ctx context.Context, request *protobuf.StreamStarted) (*protobuf.Status, error) {
	mutex.Lock()
	defer mutex.Unlock()

	//get all values from the request
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

	//handle the light, camera and delete silences
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

	//this goroutine sets the stream to live and gives the hls link free to TUMLive via the edgeServer
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

		//get the hls url based on what the edge server is set
		hlsUrl := fmt.Sprintf("%v/%v", tools.Cfg.Edge.Domain, request.HLSUrl)

		time.Sleep(time.Second * 5)

		//check if the hls url is ok, if not, sentry will capture the error
		if !isHLSUrlOk(hlsUrl) {
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetExtra("URL", request.HLSUrl)
				scope.SetExtra("StreamID", request.StreamID)
				scope.SetExtra("LectureHall", stream.LectureHallID)
				scope.SetExtra("Runner", runner.Hostname)
				scope.SetExtra("Version", request.SourceType)
				sentry.CaptureException(errors.New("DVR URL 404s"))
			})
			hlsUrl = strings.ReplaceAll(hlsUrl, "?dvr", "")
		}

		//save the hls url to the stream
		switch request.Version {
		case "CAM":
			g.StreamsDao.SaveCAMURL(&stream, hlsUrl)
		case "PRES":
			g.StreamsDao.SavePRESURL(&stream, hlsUrl)
		default:
			g.StreamsDao.SaveCOMBURL(&stream, hlsUrl)
		}

		//notify on the webpage that the stream is live
		NotifyViewersLiveState(stream.Model.ID, true)
		NotifyLiveUpdateCourseWentLive(stream.Model.ID)
	}()
	return &protobuf.Status{Ok: true}, nil
}

// isHLSUrlOk checks if the hls url is ok. copy from the worker version
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

// NotifyVOdUploadFinished is called by the runner when the upload of a vod is finished
func (g GrpcRunnerServer) NotifyVoDUploadFinished(ctx context.Context, request *protobuf.VoDUploadFinished) (*protobuf.Status, error) {

	panic("implement!")
}

// NotifyActionFinished is so TUMLive gets notified on the completion of an action. This is set generic so that there are not more notify functions on the runner part
func (g GrpcRunnerServer) NotifyActionFinished(ctx context.Context, request *protobuf.ActionFinished) (*protobuf.Status, error) {

	//get the runner from the data bank. Right now not used, but
	//_, err := g.RunnerDao.Get(ctx, request.RunnerID)

	//Checks by type of action how to handle the finished action. This was meant for when a stream needs to be transcoded or uploaded
	switch request.Type {
	case "Upload":
		status, err := SetUploadFinished(ctx, request)
		return status, err
	case "Transcode":
		status, err := SetTranscodeFinished(ctx, request)
		return status, err
	case "Stream":
	}

	return &protobuf.Status{Ok: true}, nil

}

func SetUploadFinished(ctx context.Context, req *protobuf.ActionFinished) (*protobuf.Status, error) {
	panic("implement me")
}

func SetTranscodeFinished(ctx context.Context, req *protobuf.ActionFinished) (*protobuf.Status, error) {
	panic("implement me")
}

/*
NotifyForStreams is a CronFunction that is called every 2 minutes to check if there are streams that are due to start in 10 minutes or less.
it creates a Job (a model that is used to keep a bundle of actions together and keep individual tasks) for designated tasks
*/
func NotifyForStreams(dao dao.DaoWrapper) func() {
	return func() {
		//get all streams that are due to start in 10 minutes or less
		streams := dao.StreamsDao.GetDueStreamsForWorkers()

		//for each stream, create a fitting Job
		for i := range streams {

			//makes sure the stream is not failing before
			err := dao.StreamsDao.SaveEndedState(streams[i].ID, false)
			if err != nil {
				logger.Warn("Can't save ended state", err)
				sentry.CaptureException(err)
				continue
			}

			//Get all the values for the stream necessary
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

			//depending on what type of stream it is, it creates a job for the runner
			switch courseForStream.GetSourceModeForLectureHall(streams[i].LectureHallID) {
			case 1: //Presentation
				values["version"] = "PRES"
				values["source"] = lectureHallForStream.PresIP
				err = CreateJob(dao, ctx, values) //presentation
				if err != nil {
					logger.Error("Can't create job", err)
				}
				break
			case 2: //Camera
				values["version"] = "CAM"
				values["source"] = lectureHallForStream.CamIP
				err = CreateJob(dao, ctx, values)
				if err != nil {
					logger.Error("Can't create job", err)
				}
				break
			default: //Combined. means all three streams are needed
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

		ctx := context.Background()

		//checking for each running action if the runner is still doing the job or if it is dead
		activeAction, err := dao.ActionDao.GetRunningActions(ctx)
		if err != nil {
			logger.Error("Can't get running actions", err)
		}
		for _, action := range activeAction {
			//if the action is 5 minutes past its end, it is set to ignored and needs to be reevaluated manually
			if action.End.Before(time.Now().Add(-5 * time.Minute)) {
				action.SetToIgnored()
				err = dao.ActionDao.UpdateAction(ctx, &action)
				logger.Info("Action ignored, check for progress manually", "action", action.ID)
				continue
			}
			//get the runner that is currently working on the action
			runner, err := action.GetCurrentRunner()
			if err != nil {
				logger.Error("Can't get current runner", err)
				action.SetToFailed()
				err = dao.ActionDao.UpdateAction(ctx, &action)
				if err != nil {
					return
				}
				continue
			}
			//check if the runner is dead, if the action is not completed and if the runner is still assigned to the action
			//if so, it will be set to failed and needs to be restarted. This can be later set in one forLoop
			hasAction := strings.Contains(runner.Actions, strconv.Itoa(int(action.ID)))
			if !runner.IsAlive() && !action.IsCompleted() && hasAction {
				action.SetToFailed()
				err = dao.ActionDao.UpdateAction(ctx, &action)
				if err != nil {
					return
				}
			}
		}

		//Get all failed actions. this can later be set to a single loop and be thrown out.
		failedActions, err := dao.ActionDao.GetAllFailedActions(ctx)
		if err != nil {
			logger.Error("Can't get failed actions", err)
			return
		}
		for _, failedAction := range failedActions {
			//Reassign the failed actions and set them to running
			failedAction.SetToRunning()
			err := AssignRunnerAction(dao, &failedAction)
			if err != nil {
				logger.Error("Can't assign runner to action", err)
				return
			}
		}

		//Running normal jobs with the idea that they are working as they should

		//Get all jobs that have still actions left
		jobs, err := dao.JobDao.GetAllOpenJobs(ctx)
		if err != nil {
			logger.Error("Can't get jobs", err)
			return
		}
		for _, job := range jobs {
			//if these jobs are completed or have no actions, they are skipped
			if job.Actions[0].Status != 3 {
				continue
			}
			action, err := job.GetNextAction()
			if err != nil {
				logger.Error("Can't get next action", err)
				continue
			}
			if dao.JobDao.UpdateJob(ctx, job) != nil {
				logger.Error("Can't update job", err)
				continue
			}
			action.SetToRunning()
			err = AssignRunnerAction(dao, action)
			if err != nil {
				logger.Error("Can't assign runner to action", err)
				continue
			}

			err = dao.ActionDao.UpdateAction(ctx, action)
			if err != nil {
				return
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
	ctx := context.Background()
	err = dao.AssignRunner(ctx, action, &runner)
	if err != nil {
		logger.Error("Can't assign action", err)
		return err
	}
	values := map[string]interface{}{}
	err = json.Unmarshal([]byte(action.Values), &values)
	if err != nil {
		logger.Error("Can't unmarshal json", err)
		return err
	}

	switch action.Type {
	case "stream":
		StreamRequest(ctx, dao, runner, values)
		action.SetToRunning()
		break
	}
	logger.Info("runner counts", "count", len(action.AllRunners))
	err = dao.ActionDao.UpdateAction(ctx, action)
	if err != nil {
		logger.Error("Can't update action", err)
		return err
	}
	return nil
}

func CreateJob(dao dao.DaoWrapper, ctx context.Context, values map[string]interface{}) error {
	logger.Info("Creating Job", "values", values)
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
			End:    values["end"].(time.Time),
		})
		job.Actions = append(job.Actions, actions...)
		break
	case "transcode":
		actions = append(actions, model.Action{
			Status: 3,
			Type:   "transcode",
			Values: string(value),
			End:    values["end"].(time.Time),
		})
		job.Actions = append(job.Actions, actions...)
		break
	case "upload":
		actions = append(actions, model.Action{
			Status: 3,
			Type:   "upload",
			Values: string(value),
			End:    values["end"].(time.Time),
		})
		job.Actions = append(job.Actions, actions...)
		break
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
		logger.Error("Failed to init grpc server", "err", err)
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
			logger.Error("Can't serve grpc", "err", err)
		}
	}()
}
