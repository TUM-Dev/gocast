package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// interactively test your implementation here
func main() {
	c, err := dialIn("localhost")
	if err != nil {
		log.Fatal(err)
	}
	client := pb.NewToWorkerClient(c)
	waveform, err := client.RequestWaveform(context.Background(), &pb.WaveformRequest{
		File:     "/srv/cephfs/livestream/rec/TUM-Live/2021/W/fpv/2021-10-22_08-30/fpv-2021-10-22-08-30COMB.mp4",
		WorkerId: "abc",
	})
	if err != nil {
		log.Fatal(err)
	}
	e := base64.StdEncoding.EncodeToString(waveform.Waveform)
	fmt.Println(e)
}

func dialIn(host string) (*grpc.ClientConn, error) {
	credentials := insecure.NewCredentials()
	log.Info("Connecting to:" + fmt.Sprintf("%s:50051", host))
	conn, err := grpc.Dial(fmt.Sprintf("%s:50051", host), grpc.WithTransportCredentials(credentials))
	return conn, err
}
