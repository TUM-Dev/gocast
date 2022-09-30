package stats

import (
	"context"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"github.com/joschahenningsen/TUM-Live/model"
	"time"
)

type Stats struct {
	client    influxdb2.Client
	liveStats api.WriteAPIBlocking
}

var Client *Stats

func InitStats(client influxdb2.Client) {
	Client = &Stats{
		client:    client,
		liveStats: client.WriteAPIBlocking("rbg", "live_stats"),
	}
}

func (s *Stats) AddStreamStat(courseId string, stat model.Stat) error {
	p := influxdb2.NewPoint("viewers",
		map[string]string{"live": fmt.Sprintf("%v", stat.Live), "stream": fmt.Sprintf("%d", stat.StreamID), "course": courseId},
		map[string]interface{}{"num": stat.Viewers},
		time.Now())
	return s.liveStats.WritePoint(context.Background(), p)
}
