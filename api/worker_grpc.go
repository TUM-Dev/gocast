package api

// worker_grpc.go handles communication with workers via grpc
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	go_anel_pwrctrl "github.com/RBG-TUM/go-anel-pwrctrl"
	"github.com/getsentry/sentry-go"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/camera"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

var mutex = sync.Mutex{}

var lightIndices = []int{0, 1, 2} // turn on all 3 outlets. TODO: make configurable

type server struct {
	pb.UnimplementedFromWorkerServer
}

func dialIn(targetWorker model.Worker) (*grpc.ClientConn, error) {
	credentials := insecure.NewCredentials()
	log.Info("Connecting to:" + fmt.Sprintf("%s:50051", targetWorker.Host))
	conn, err := grpc.Dial(fmt.Sprintf("%s:50051", targetWorker.Host), grpc.WithTransportCredentials(credentials))
	return conn, err
}

func endConnection(conn *grpc.ClientConn) {
	if err := conn.Close(); err != nil {
		log.WithError(err).Error("Could not close connection to worker")
	}
}

// JoinWorkers is a request from a worker to join the pool. On success, the workerID is returned.
func (s server) JoinWorkers(ctx context.Context, request *pb.JoinWorkersRequest) (*pb.JoinWorkersResponse, error) {
	log.WithField("host", request.Hostname).Info("JoinWorkers called")
	if request.Token != tools.Cfg.WorkerToken {
		return nil, status.Error(codes.Unauthenticated, "Invalid token")
	}

	worker, err := dao.GetWorkerByHostname(ctx, request.Hostname)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, status.Errorf(codes.Internal, "get worker by hostname: %v", err)
	}
	if err == nil {
		// worker already exists, return its ID
		return &pb.JoinWorkersResponse{
			WorkerId: worker.WorkerID,
		}, nil
	}

	// worker does not exist, create it
	worker = model.Worker{
		Host:     request.Hostname,
		WorkerID: uuid.NewV4().String(),
		LastSeen: time.Now(),
	}
	if err := dao.CreateWorker(&worker); err != nil {
		log.WithError(err).Error("Could not add worker to database")
		return nil, status.Errorf(codes.Internal, "Could not add worker to database")
	}
	log.Info("Added worker to database")
	return &pb.JoinWorkersResponse{
		WorkerId: worker.WorkerID,
	}, nil
}

//NotifySilenceResults handles the results of silence detection sent by a worker
func (s server) NotifySilenceResults(ctx context.Context, request *pb.SilenceResults) (*pb.Status, error) {
	if _, err := dao.GetWorkerByID(ctx, request.WorkerID); err != nil {
		return nil, err
	}
	if _, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", request.GetStreamID())); err != nil {
		return nil, err
	}
	var silences []model.Silence
	for i := range request.Starts {
		silences = append(silences, model.Silence{
			Start:    uint(request.Starts[i]),
			End:      uint(request.Ends[i]),
			StreamID: uint(request.StreamID),
		})
	}
	if len(silences) == 0 {
		return &pb.Status{Ok: true}, nil
	}
	if err := dao.UpdateSilences(silences, fmt.Sprintf("%d", request.StreamID)); err != nil {
		return nil, err
	}
	return &pb.Status{Ok: true}, nil

}

// SendSelfStreamRequest handles the request from a worker when a stream starts publishing via obs, etc.
// returns an error if anything goes wrong OR the stream may not be published.
func (s server) SendSelfStreamRequest(ctx context.Context, request *pb.SelfStreamRequest) (*pb.SelfStreamResponse, error) {
	if _, err := dao.GetWorkerByID(ctx, request.GetWorkerID()); err != nil {
		return nil, err
	}
	if request.StreamKey == "" {
		return nil, errors.New("stream key empty")
	}
	stream, err := dao.GetStreamByKey(ctx, request.StreamKey)
	if err != nil {
		return nil, err
	}
	course, err := dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return nil, err
	}
	if request.CourseSlug != fmt.Sprintf("%s-%d", course.Slug, stream.ID) {
		return nil, fmt.Errorf("bad stream name, should: %s, is: %s", fmt.Sprintf("%s-%d", course.Slug, stream.ID), request.CourseSlug)
	}
	// reject streams that are more than 30 minutes in the future or more than 30 minutes past
	if !(time.Now().After(stream.Start.Add(time.Minute*-30)) && time.Now().Before(stream.End.Add(time.Minute*30))) {
		log.WithFields(log.Fields{"streamId": stream.ID}).Warn("Stream rejected, time out of bounds")
		return nil, errors.New("stream rejected")
	}
	ingestServer, err := dao.GetBestIngestServer()
	if err != nil {
		return nil, err
	}
	slot, err := dao.GetStreamSlot(ingestServer.ID)
	if err != nil {
		return nil, err
	}
	slot.StreamID = stream.ID
	dao.SaveSlot(slot)

	return &pb.SelfStreamResponse{
		StreamID:     uint32(stream.ID),
		CourseSlug:   course.Slug,
		CourseYear:   uint32(course.Year),
		StreamStart:  timestamppb.New(stream.Start),
		CourseTerm:   course.TeachingTerm,
		UploadVoD:    course.VODEnabled,
		IngestServer: ingestServer.Url,
		StreamName:   slot.StreamName,
		OutUrl:       ingestServer.OutUrl,
	}, nil
}

var lightLock = sync.Mutex{}

// NotifyStreamStart handles workers notification about streams being started
// Deprecated: this is now "NotifyStreamStarted"
func (s server) NotifyStreamStart(ctx context.Context, request *pb.StreamStarted) (*pb.Status, error) {
	mutex.Lock()
	defer mutex.Unlock()
	_, err := dao.GetWorkerByID(ctx, request.GetWorkerID())
	if err != nil {
		log.WithField("request", request).Warn("Got stream start with invalid WorkerID")
		return nil, err
	}
	stream, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", request.StreamID))
	if err != nil {
		log.WithError(err).Warn("Can't get stream by ID to set live")
		return nil, err
	}
	stream.LiveNow = true
	switch request.GetSourceType() {
	case "CAM":
		stream.PlaylistUrlCAM = request.HlsUrl
	case "PRES":
		stream.PlaylistUrlPRES = request.HlsUrl
	default:
		stream.PlaylistUrl = request.HlsUrl
	}
	err = dao.SaveStream(&stream)
	if err != nil {
		log.WithError(err).Error("Can't save stream when setting live")
		return nil, err
	}
	return nil, nil
}

//NotifyStreamFinished handles workers notification about streams being finished
func (s server) NotifyStreamFinished(ctx context.Context, request *pb.StreamFinished) (*pb.Status, error) {
	if _, err := dao.GetWorkerByID(ctx, request.GetWorkerID()); err != nil {
		return nil, errors.New("authentication failed: invalid worker id")
	} else {
		stream, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", request.StreamID))
		if err != nil {
			log.WithError(err).Error("Can't find stream to set not live")
		} else {
			go func() {
				err := handleLightOffSwitch(stream)
				if err != nil {
					log.WithError(err).Error("Can't handle light off switch")
				}
				err = dao.SaveEndedState(stream.ID, true)
				if err != nil {
					log.WithError(err).Error("Can't set stream done")
				}
			}()
		}
		// wait 2 hours to clear the dvr cache
		go func() {
			time.Sleep(time.Hour * 2)
			err := dao.RemoveStreamFromSlot(stream.ID)
			if err != nil {
				log.WithError(err).Error("Can't remove stream from streamName")
			}
		}()

		// wait 2 hours to clear the dvr cache
		go func() {
			time.Sleep(time.Hour * 2)
			err := dao.RemoveStreamFromSlot(stream.ID)
			if err != nil {
				log.WithError(err).Error("Can't remove stream from streamName")
			}
		}()

		err = dao.SetStreamNotLiveById(uint(request.StreamID))
		if err != nil {
			log.WithError(err).Error("Can't set stream not live")
		}
		NotifyViewersLiveState(uint(request.StreamID), false)
	}
	return &pb.Status{Ok: true}, nil
}

func handleCameraPositionSwitch(stream model.Stream) error {
	if stream.LectureHallID == 0 {
		return nil
	}
	course, err := dao.GetCourseById(context.Background(), stream.CourseID)
	if err != nil {
		return err
	}
	lectureHall, err := dao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		return err
	}
	var preferences []model.CameraPresetPreference
	// make sure there is an empty list if no preferences are found (null or empty string in db)
	if course.CameraPresetPreferences == "" {
		course.CameraPresetPreferences = "[]"
	}
	err = json.Unmarshal([]byte(course.CameraPresetPreferences), &preferences)
	if err != nil {
		return err
	}
	for _, preference := range preferences {
		if preference.LectureHallID == stream.LectureHallID {
			return camera.NewCamera(lectureHall.CameraIP, tools.Cfg.Auths.CamAuth).SetPreset(preference.PresetID)
		}
	}
	// no preset found for this lecture hall, use default
	defaultPreset, err := dao.NewCameraPresetDao().GetDefaultCameraPreset(lectureHall.ID)
	if err != nil {
		return err
	}
	return camera.NewCamera(lectureHall.CameraIP, tools.Cfg.Auths.CamAuth).SetPreset(defaultPreset.PresetID)
}

func handleLightOnSwitch(stream model.Stream) error {
	if stream.LectureHallID == 0 {
		return nil // no light to switch
	}
	lectureHall, err := dao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		return err
	}
	lightLock.Lock()
	defer lightLock.Unlock()

	var errs []error
	client := go_anel_pwrctrl.New(lectureHall.PwrCtrlIp, tools.Cfg.Auths.PwrCrtlAuth)
	for _, lightIndex := range lightIndices {
		err := client.TurnOn(lightIndex)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("can't turn on lights: %v", errs)
	}
	return nil
}

func handleLightOffSwitch(stream model.Stream) error {
	if stream.LectureHallID == 0 {
		return nil // no light to switch
	}
	lightLock.Lock()
	defer lightLock.Unlock()
	liveStreamsInLectureHall, err := dao.GetLiveStreamsInLectureHall(stream.LectureHallID)
	if err != nil {
		return err
	}
	if len(liveStreamsInLectureHall) > 1 {
		return nil // another stream is live, don't turn off the light
	}
	if len(liveStreamsInLectureHall) == 1 && liveStreamsInLectureHall[0].ID != stream.ID {
		return nil // the one different live stream is not this one, don't turn off the light
	}
	lectureHall, err := dao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		return err
	}
	client := go_anel_pwrctrl.New(lectureHall.PwrCtrlIp, tools.Cfg.Auths.PwrCrtlAuth)
	for _, index := range lightIndices {
		err := client.TurnOff(index)
		if err != nil {
			return err
		}
	}

	return nil
}

//SendHeartBeat receives heartbeat messages sent by workers
func (s server) SendHeartBeat(ctx context.Context, request *pb.HeartBeat) (*pb.Status, error) {
	if worker, err := dao.GetWorkerByID(ctx, request.GetWorkerID()); err != nil {
		return nil, errors.New("authentication failed: invalid worker id")
	} else {
		worker.Workload = uint(request.Workload)
		worker.LastSeen = time.Now()
		worker.Status = strings.Join(request.Jobs, ", ")
		worker.CPU = request.CPU
		worker.Memory = request.Memory
		worker.Disk = request.Disk
		worker.Uptime = request.Uptime
		worker.Version = request.Version
		err := dao.SaveWorker(worker)
		if err != nil {
			return nil, err
		}
		return &pb.Status{Ok: true}, nil
	}
}

//NotifyTranscodingFinished receives and handles messages from workers about finished transcoding
func (s server) NotifyTranscodingFinished(ctx context.Context, request *pb.TranscodingFinished) (*pb.Status, error) {
	if _, err := dao.GetWorkerByID(ctx, request.WorkerID); err != nil {
		return nil, err
	}
	stream, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", request.StreamID))
	if err != nil {
		return nil, err
	}
	// look for file to prevent duplication
	shouldAddFile := true
	for _, file := range stream.Files {
		if file.Path == request.FilePath {
			shouldAddFile = false
			break
		}
	}
	if shouldAddFile {
		stream.Files = append(stream.Files, model.File{StreamID: stream.ID, Path: request.FilePath})
	}

	if request.Duration != 0 {
		stream.Duration = request.Duration
	}
	stream.StreamStatus = model.StatusConverted
	err = dao.SaveStream(&stream)
	if err != nil {
		log.WithError(err).Error("Can't save stream")
		return nil, err
	}
	return &pb.Status{Ok: true}, nil
}

//NotifyUploadFinished receives and handles messages from workers about finished uploads
func (s server) NotifyUploadFinished(ctx context.Context, req *pb.UploadFinished) (*pb.Status, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if _, err := dao.GetWorkerByID(ctx, req.WorkerID); err != nil {
		return nil, err
	}
	stream, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", req.StreamID))
	if err != nil {
		return nil, err
	}
	if stream.LiveNow {
		log.WithField("req", req).Warn("VoD not saved, stream is live.")
		return nil, nil
	}
	stream.Recording = true
	stream.StreamStatus = model.StatusUnknown
	switch req.SourceType {
	case "CAM":
		stream.PlaylistUrlCAM = req.HLSUrl
	case "PRES":
		stream.PlaylistUrlPRES = req.HLSUrl
	default:
		stream.PlaylistUrl = req.HLSUrl
	}
	if err = dao.SaveStream(&stream); err != nil {
		return nil, err
	}
	return &pb.Status{Ok: true}, nil
}

// GetStreamInfoForUpload returns the stream info for a stream identified by its upload token.
// after calling, the token is deleted.
func (s server) GetStreamInfoForUpload(ctx context.Context, request *pb.GetStreamInfoForUploadRequest) (*pb.GetStreamInfoForUploadResponse, error) {
	_, err := dao.GetWorkerByID(ctx, request.WorkerID)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid worker id")
	}
	key, err := dao.NewUploadKeyDao().GetUploadKey(request.UploadKey)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "key not found")
	}
	course, err := dao.GetCourseById(ctx, key.Stream.CourseID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "course not found")
	}
	err = dao.NewUploadKeyDao().DeleteUploadKey(key)
	if err != nil {
		log.WithError(err).Error("Can't delete upload key")
	}
	return &pb.GetStreamInfoForUploadResponse{
		CourseSlug:  course.Slug,
		CourseTerm:  course.TeachingTerm,
		CourseYear:  uint32(course.Year),
		StreamStart: timestamppb.New(key.Stream.Start),
		StreamEnd:   timestamppb.New(key.Stream.End),
		StreamID:    uint32(key.StreamID),
	}, nil
}

//NotifyStreamStarted receives stream started events from workers
func (s server) NotifyStreamStarted(ctx context.Context, request *pb.StreamStarted) (*pb.Status, error) {
	mutex.Lock()
	defer mutex.Unlock()
	worker, err := dao.GetWorkerByID(ctx, request.WorkerID)
	if err != nil {
		return nil, err
	}
	stream, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", request.GetStreamID()))
	if err != nil {
		log.WithError(err).Println("Can't find stream")
		return nil, err
	}
	go func() {
		err := handleLightOnSwitch(stream)
		if err != nil {
			log.WithError(err).Error("Can't handle light on switch")
		}
		err = handleCameraPositionSwitch(stream)
		if err != nil {
			log.WithError(err).Error("Can't handle camera position switch")
		}
	}()
	go func() {
		// interims solution; sometimes dvr doesn't work as expected.
		// here we check if the url 404s and remove dvr from the stream in that case
		stream.LiveNow = true
		time.Sleep(time.Second * 5)
		if !isHlsUrlOk(request.HlsUrl) {
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetExtra("URL", request.HlsUrl)
				scope.SetExtra("StreamID", request.StreamID)
				scope.SetExtra("LectureHall", stream.LectureHallID)
				scope.SetExtra("Worker", worker.Host)
				scope.SetExtra("Version", request.SourceType)
				sentry.CaptureException(errors.New("DVR URL 404s"))
			})
			request.HlsUrl = strings.ReplaceAll(request.HlsUrl, "?dvr", "")
		}

		switch request.GetSourceType() {
		case "CAM":
			dao.SaveCAMURL(&stream, request.HlsUrl)
		case "PRES":
			dao.SavePRESURL(&stream, request.HlsUrl)
		default:
			dao.SaveCOMBURL(&stream, request.HlsUrl)
		}
		NotifyViewersLiveState(stream.Model.ID, true)
	}()

	return &pb.Status{Ok: true}, nil
}

func isHlsUrlOk(url string) bool {
	r, err := http.Get(url)
	if err != nil {
		return false
	}
	all, err := ioutil.ReadAll(r.Body)
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

// NotifyWorkers collects all streams that are due to stream
// (starts in the next 10 minutes from a lecture hall)
// and invokes the corresponding calls at the workers with the least workload via gRPC
func NotifyWorkers() {
	notifyWorkersPremieres()
	streams := dao.GetDueStreamsForWorkers()
	workers := dao.GetAliveWorkers()
	if len(workers) == 0 && len(streams) != 0 {
		log.Error("not enough workers to handle streams")
		return
	}
	for i := range streams {
		err := dao.SaveEndedState(streams[i].ID, false)
		if err != nil {
			log.WithError(err).Warn("Can't set stream undone")
			sentry.CaptureException(err)
			continue
		}
		courseForStream, err := dao.GetCourseById(context.Background(), streams[i].CourseID)
		if err != nil {
			log.WithError(err).Warn("Can't get course for stream, skipping")
			sentry.CaptureException(err)
			continue
		}
		lectureHallForStream, err := dao.GetLectureHallByID(streams[i].LectureHallID)
		if err != nil {
			log.WithError(err).Error("Can't get lecture hall for stream, skipping")
			sentry.CaptureException(err)
			continue
		}
		sources := []string{lectureHallForStream.CombIP, lectureHallForStream.PresIP, lectureHallForStream.CamIP}
		for sourceNum, source := range sources {
			if source == "" {
				continue
			}
			var sourceType string
			if sourceNum == 1 {
				sourceType = "PRES"
			} else if sourceNum == 2 {
				sourceType = "CAM"
			} else {
				sourceType = "COMB"
			}
			server, err := dao.GetBestIngestServer()
			if err != nil {
				log.WithError(err).Error("Can't find ingest server")
				continue
			}
			var slot model.StreamName
			if sourceType == "COMB" { //try to find a transcoding slot for comb view:
				slot, err = dao.GetTranscodedStreamSlot(server.ID)
			}
			if sourceType != "COMB" || err != nil {
				slot, err = dao.GetStreamSlot(server.ID)
				if err != nil {
					log.WithError(err).Error("No free stream slot")
					continue
				}
			}
			slot.StreamID = streams[i].ID
			dao.SaveSlot(slot)
			req := pb.StreamRequest{
				SourceType:    sourceType,
				SourceUrl:     source,
				CourseSlug:    courseForStream.Slug,
				Start:         timestamppb.New(streams[i].Start),
				End:           timestamppb.New(streams[i].End),
				PublishStream: courseForStream.LiveEnabled,
				PublishVoD:    courseForStream.VODEnabled,
				StreamID:      uint32(streams[i].ID),
				CourseTerm:    courseForStream.TeachingTerm,
				CourseYear:    uint32(courseForStream.Year),
				StreamName:    slot.StreamName,
				IngestServer:  server.Url,
				OutUrl:        server.OutUrl,
			}
			workerIndex := getWorkerWithLeastWorkload(workers)
			workers[workerIndex].Workload += 3
			err = dao.SaveWorkerForStream(streams[i], workers[workerIndex])
			if err != nil {
				log.WithError(err).Error("Could not save worker for stream")
				return
			}
			conn, err := dialIn(workers[workerIndex])
			if err != nil {
				log.WithError(err).Error("Unable to dial server")
				workers[workerIndex].Workload -= 1 // decrease workers load only by one (backoff)
				continue
			}
			client := pb.NewToWorkerClient(conn)
			req.WorkerId = workers[workerIndex].WorkerID
			resp, err := client.RequestStream(context.Background(), &req)
			if err != nil || !resp.Ok {
				log.WithError(err).Error("could not assign stream!")
				workers[workerIndex].Workload -= 1 // decrease workers load only by one (backoff)
			}
			endConnection(conn)
		}
	}
}

//notifyWorkersPremieres looks for premieres that should be streamed and assigns them to workers.
func notifyWorkersPremieres() {
	streams := dao.GetDuePremieresForWorkers()
	workers := dao.GetAliveWorkers()

	if len(workers) == 0 && len(streams) != 0 {
		log.Error("Not enough alive workers for premiere")
		return
	}
	for i := range streams {
		err := dao.SaveEndedState(streams[i].ID, false)
		if err != nil {
			log.WithError(err).Warn("Can't set stream undone")
			sentry.CaptureException(err)
			continue
		}
		if len(streams[i].Files) == 0 {
			log.WithField("streamID", streams[i].ID).Warn("Request to self stream without file")
			continue
		}
		workerIndex := getWorkerWithLeastWorkload(workers)
		workers[workerIndex].Workload += 3
		ingestServer, err := dao.GetBestIngestServer()
		if err != nil {
			log.WithError(err).Error("Can't find ingest server")
			continue
		}
		req := pb.PremiereRequest{
			StreamID:     uint32(streams[i].ID),
			FilePath:     streams[i].Files[0].Path,
			IngestServer: ingestServer.Url,
			OutUrl:       ingestServer.OutUrl,
		}
		conn, err := dialIn(workers[workerIndex])
		if err != nil {
			log.WithError(err).Error("Unable to dial server")
			endConnection(conn)
			workers[workerIndex].Workload -= 1
			continue
		}
		client := pb.NewToWorkerClient(conn)
		req.WorkerID = workers[workerIndex].WorkerID
		resp, err := client.RequestPremiere(context.Background(), &req)
		if err != nil || !resp.Ok {
			log.WithError(err).Error("could not assign premiere!")
			workers[workerIndex].Workload -= 1
		}
		endConnection(conn)
	}
}

// NotifyWorkersToStopStream notifies all workers for a given stream to quit encoding
func NotifyWorkersToStopStream(stream model.Stream, discardVoD bool) {
	workers, err := dao.GetWorkersForStream(stream)
	if err != nil {
		log.WithError(err).Error("Could not get workers for stream")
		return
	}

	if len(workers) == 0 {
		log.Error("No workers for stream found")
		return
	}

	// Iterate over all workers that are used for the given stream
	for _, currentWorker := range workers {
		req := pb.EndStreamRequest{
			StreamID:   uint32(stream.ID),
			WorkerID:   currentWorker.WorkerID,
			DiscardVoD: discardVoD,
		}
		conn, err := dialIn(currentWorker)
		if err != nil {
			log.WithError(err).Error("Unable to dial server")
			continue
		}
		client := pb.NewToWorkerClient(conn)
		resp, err := client.RequestStreamEnd(context.Background(), &req)
		if err != nil || !resp.Ok {
			log.WithError(err).Error("Could not end stream")
		}
		endConnection(conn)
	}

	// All workers for stream are assumed to be done
	err = dao.ClearWorkersForStream(stream)
	if err != nil {
		log.WithError(err).Error("Could not delete workers for stream")
		return
	}
}

//getWorkerWithLeastWorkload Gets the index of the worker from workers with the least workload.
//workers must not be empty!
func getWorkerWithLeastWorkload(workers []model.Worker) int {
	foundWorker := 0
	for i := range workers {
		if workers[i].Workload < workers[foundWorker].Workload {
			foundWorker = i
		}
	}
	return foundWorker
}

// init initializes a gRPC server on port 50052
func init() {
	log.Info("Serving heartbeat")
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.WithError(err).Error("Failed to init grpc server")
		return
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	pb.RegisterFromWorkerServer(grpcServer, &server{})
	reflection.Register(grpcServer)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.WithError(err).Errorf("Can't serve grpc")
		}
	}()
}
