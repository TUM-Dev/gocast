// Package api_grpc Handles communication with workers
package api_grpc

import (
	"TUM-Live/api"
	"TUM-Live/dao"
	"TUM-Live/model"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/joschahenningsen/TUM-Live-Worker-v2/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net"
	"strconv"
	"strings"
	"time"
)

type server struct {
	pb.UnimplementedFromWorkerServer
}

func (s server) NotifySilenceResults(ctx context.Context, request *pb.SilenceResults) (*pb.Status, error) {
	if _, err := dao.GetWorkerByID(ctx, request.WorkerID); err != nil {
		return nil, err
	}
	if _, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", request.GetStreamID())); err == nil {
		var silences []model.Silence
		for i, _ := range request.Starts {
			silences = append(silences, model.Silence{
				Start:    uint(request.Starts[i]),
				End:      uint(request.Ends[i]),
				StreamID: uint(request.StreamID),
			})
		}
		if len(silences) == 0 {
			return &pb.Status{Ok: true}, nil
		}
		if err = dao.UpdateSilences(silences, fmt.Sprintf("%d", request.StreamID)); err != nil {
			return nil, err
		}
		return &pb.Status{Ok: true}, nil
	} else {
		return nil, err
	}
}

// SendSelfStreamRequest handles the request from a worker when a stream starts publishing via obs, etc.
// returns an error if anything goes wrong OR the stream may not be published.
func (s server) SendSelfStreamRequest(ctx context.Context, request *pb.SelfStreamRequest) (*pb.SelfStreamResponse, error) {
	if _, err := dao.GetWorkerByID(ctx, request.GetWorkerID()); err != nil {
		return nil, err
	}
	stream, err := dao.GetStreamByKey(ctx, request.StreamKey)
	if err != nil {
		return nil, err
	}
	// reject streams that are more than 30 minutes in the future or more than 30 minutes past
	if !(time.Now().After(stream.Start.Add(time.Minute*-30)) && time.Now().Before(stream.End.Add(time.Minute*30))) {
		log.WithFields(log.Fields{"streamId": stream.ID}).Warn("Stream rejected, time out of bounds")
		return nil, errors.New("stream rejected")
	}
	course, err := dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		log.WithError(err).Warn("Can't get stream for worker")
		return nil, err
	}
	return &pb.SelfStreamResponse{
		StreamID:    uint32(stream.ID),
		CourseSlug:  course.Slug,
		CourseYear:  uint32(course.Year),
		StreamStart: timestamppb.New(stream.Start),
		CourseTerm:  course.TeachingTerm,
		UploadVoD:   course.VODEnabled,
	}, nil
}

//NotifyStreamStart handles workers notification about streams being started
func (s server) NotifyStreamStart(ctx context.Context, request *pb.StreamStarted) (*pb.Status, error) {
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
		log.Printf("Got stream Finished with invalid workerID %v", request.WorkerID)
		return nil, errors.New("authentication failed: invalid worker id")
	} else {
		err := dao.SetStreamNotLiveById(strconv.Itoa(int(request.StreamID))) // todo change signature to uint
		if err != nil {
			log.Printf("Couldn't set stream not live: %v\n", err)
			sentry.CaptureException(err)
		}
		api.NotifyViewersLiveEnd(strconv.Itoa(int(request.StreamID)))
	}
	return &pb.Status{Ok: true}, nil
}

//SendHeartBeat receives heartbeat messages sent by workers
func (s server) SendHeartBeat(ctx context.Context, request *pb.HeartBeat) (*pb.Status, error) {
	if worker, err := dao.GetWorkerByID(ctx, request.GetWorkerID()); err != nil {
		log.Printf("Got heartbeat with invalid workerID %v", request.WorkerID)
		return nil, errors.New("authentication failed: invalid worker id")
	} else {
		worker.Workload = int(request.Workload)
		worker.LastSeen = time.Now()
		worker.Status = strings.Join(request.Jobs, ", ")
		dao.SaveWorker(worker)
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
	stream.Files = append(stream.Files, model.File{StreamID: stream.ID, Path: request.FilePath})
	err = dao.SaveStream(&stream)
	if err != nil {
		log.WithError(err).Error("Can't save stream")
		return nil, err
	}
	return &pb.Status{Ok: true}, nil
}

//NotifyUploadFinished receives and handles messages from workers about finished uploads
func (s server) NotifyUploadFinished(ctx context.Context, req *pb.UploadFinished) (*pb.Status, error) {
	if _, err := dao.GetWorkerByID(ctx, req.WorkerID); err != nil {
		return nil, err
	}
	stream, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", req.StreamID))
	if err != nil {
		return nil, err
	}
	stream.Recording = true
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

//NotifyStreamStarted receives stream started events from workers
func (s server) NotifyStreamStarted(ctx context.Context, request *pb.StreamStarted) (*pb.Status, error) {
	if _, err := dao.GetWorkerByID(ctx, request.WorkerID); err != nil {
		return nil, err
	}
	stream, err := dao.GetStreamByID(ctx, fmt.Sprintf("%d", request.GetStreamID()))
	if err != nil {
		log.WithError(err).Println("Can't find stream")
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
		log.WithError(err).Println("Can't set stream live")
		return nil, err
	}
	return &pb.Status{Ok: true}, nil
}

// init initializes a gRPC server on port 50052
func init() {
	log.Printf("Serving heartbeat")
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.WithError(err).Errorf("Failed to init heartbeat server %v", err)
		return
	}
	grpcServer := grpc.NewServer()
	pb.RegisterFromWorkerServer(grpcServer, &server{})
	reflection.Register(grpcServer)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Printf("Can't serve heartbeat: %v", err)
		}
	}()
}

// NotifyWorkers collects all streams that are due to stream
// (starts in the next 10 minutes from a lecture hall)
// and invokes the corresponding calls at the workers with the least workload via gRPC
// todo: split up
func NotifyWorkers() {
	streams := dao.GetDueStreamsFromLectureHalls()
	workers := dao.GetAliveWorkersOrderedByWorkload()
	if len(workers) == 0 && len(streams) != 0 {
		log.Error("not enough workers to handle streams")
		return
	}
	priorityQueue := make(PriorityQueue, len(workers))
	for i, worker := range workers {
		priorityQueue[i] = &Item{Worker: worker, Expiry: worker.Workload}
		priorityQueue[i].Index = i
	}
	heap.Init(&priorityQueue)
	var requests []pb.StreamRequest
	for i := range streams {
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
			requests = append(requests, pb.StreamRequest{
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
			})
		}
	}

	for i := range requests {
		if priorityQueue.Len() == 0 {
			log.Error("Not enough alive workers to serve stream!")
			// this would be a huge issue, notify me.
			sentry.CaptureException(errors.New("not enough alive workers to serve stream"))
			continue
		}
		item := heap.Pop(&priorityQueue).(*Item)
		conn, err := grpc.Dial(fmt.Sprintf("%s:50051", item.Worker.Host), grpc.WithInsecure())
		if err != nil {
			log.WithError(err).Error("Unable to dial server")
			sentry.CaptureException(err)
			item.Expiry += 2
			heap.Push(&priorityQueue, item)
			continue
		}

		client := pb.NewToWorkerClient(conn)
		requests[i].WorkerId = item.Worker.WorkerID
		resp, err := client.RequestStream(context.Background(), &requests[i])
		if err != nil || !resp.Ok {
			log.WithError(err).Error("could not assign stream!")
			item.Expiry += 2
			heap.Push(&priorityQueue, item)
			continue
		}
		_ = conn.Close()
		item.Expiry += 2
		heap.Push(&priorityQueue, item)
	}
}
