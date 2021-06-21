// Package worker Handles communication with workers
package worker

import (
	"TUM-Live/dao"
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/joschahenningsen/TUM-Live-Worker-v2/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"strings"
	"time"
)

type server struct {
	pb.UnimplementedHeartbeatServer
}

//SendHeartBeat serves heartbeat messages sent by workers
func (s server) SendHeartBeat(ctx context.Context, request *pb.HeartBeat) (*pb.Status, error) {
	if worker, err := dao.GetWorkerByID(ctx, request.GetWorkerID()); err != nil {
		log.Printf("Got heartbeat with invalid workerID %v", request.WorkerID)
		return nil, err
	} else {
		worker.Workload = int(request.Workload)
		worker.LastSeen = time.Now()
		worker.Status = strings.Join(request.Jobs, " - ")
		dao.SaveWorker(worker)
		return &pb.Status{Ok: true}, nil
	}
}

// init initializes a gRPC server on port 50052 which is routed to :443/worker
func init() {
	log.Printf("Serving heartbeat")
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Printf("Failed to init heartbeat server %v", err)
		return
	}
	grpcServer := grpc.NewServer()
	pb.RegisterHeartbeatServer(grpcServer, &server{})
	reflection.Register(grpcServer)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Printf("Can't serve heartbeat: %v", err)
		}
	}()
}

func NotifyWorkersProto() {
	streams := dao.GetDueStreamsFromLectureHalls()
	workers := dao.GetAliveWorkersOrderedByWorkload()
	if len(workers) == 0 {
		return
	}
	//todo
	var requests []pb.StreamRequest
	for i := range streams {

	}

	for i, stream := range streams {
		assignedWorker := workers[i%len(workers)]
		lectureHall, err := dao.GetLectureHallByID(stream.LectureHallID)
		course, _ := dao.GetCourseById(context.Background(), stream.CourseID)
		if err != nil {
			sentry.CaptureException(err)
			continue
		}
		sources := make(map[string]string)
		if lectureHall.CamIP != "" {
			sources["CAM"] = lectureHall.CamIP
		}
		if lectureHall.PresIP != "" {
			sources["PRES"] = lectureHall.PresIP
		}
		if lectureHall.CombIP != "" {
			sources["COMB"] = lectureHall.CombIP
		}
		conn, err := grpc.Dial(fmt.Sprintf("%s:50051", assignedWorker.Host), grpc.WithInsecure())
		if err != nil {
			log.Printf("Unable to dial server %v", err)
			continue
		}

		_ = conn.Close()
		client := pb.NewStreamClient(conn)
		resp, err := client.RequestStream(context.Background(), &pb.StreamRequest{
			WorkerId:   "",
			SourceType: "COMB",
			SourceUrl:  sources["COMB"],
			CourseSlug: course.Slug,
			Year:       2021,
		})
		if err != nil {
			log.Printf("failed to request stream from client")
			continue
		}
		if !resp.Ok {
			log.Printf("response not ok")
		}
	}
}

func assignStreamToWorkerWithLeastLoad(request *pb.StreamRequest) {
	workers := dao.GetAliveWorkersOrderedByWorkload()
}
