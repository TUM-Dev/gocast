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
	bucket    string
	client    influxdb2.Client
	liveStats api.WriteAPI
	query     api.QueryAPI
}

var Client *Stats

func InitStats(client influxdb2.Client) {
	bucket := "live_stats"
	Client = &Stats{
		bucket:    bucket,
		client:    client,
		liveStats: client.WriteAPI("rbg", bucket),
		query:     client.QueryAPI("rbg"),
	}
}

func (s *Stats) AddStreamStat(courseId string, stat model.Stat) {
	p := influxdb2.NewPoint("viewers",
		map[string]string{"live": fmt.Sprintf("%v", stat.Live), "stream": fmt.Sprintf("%d", stat.StreamID), "course": courseId},
		map[string]interface{}{"num": stat.Viewers},
		time.Now())
	s.liveStats.WritePoint(p)
	s.liveStats.Flush()
}

func (s *Stats) AddStreamVODStat(courseId string, streamId string) {
	p := influxdb2.NewPoint("viewers",
		map[string]string{"live": "false", "stream": streamId, "course": courseId},
		map[string]interface{}{"num": 1},
		time.Now())

	s.liveStats.WritePoint(p)
	s.liveStats.Flush()
}

// GetCourseNumVodViews returns the sum of vod views of a course
func (s *Stats) GetCourseNumVodViews(courseID uint, from time.Time, to time.Time) (int, error) {
	query := fmt.Sprintf(`from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "false")
	|> keep(columns: ["_value"])
	|> sum()`, from.Unix(), to.Unix(), courseID)

	if res, err := s.query.Query(context.Background(), query); err != nil {
		return 0, err
	} else {
		return parseValue(res.Record().Value(), 0), nil
	}
}

// GetCourseNumLiveViews returns the sum of live views of a course based on the maximum views per lecture
func (s *Stats) GetCourseNumLiveViews(courseID uint, from time.Time, to time.Time) (int, error) {
	query := fmt.Sprintf(`from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "true") 
	|> group(columns: ["stream"])
	|> sum(column: "_value")
	|> keep(columns: ["_value"])
	|> max()`, from.Unix(), to.Unix(), courseID)

	if res, err := s.query.Query(context.Background(), query); err != nil {
		return 0, err
	} else {
		return parseValue(res.Record().Value(), 0), nil
	}
}

type TimeValueEntry struct {
	Time time.Time
	Val  int
}

type TimeValues struct {
	Entries []TimeValueEntry
}

func (t *TimeValues) GetChartJsData() []map[string]any {
	var data = make([]map[string]any, 0)
	for _, entry := range t.Entries {
		data = append(data, map[string]any{
			"x": entry.Time.Format("2006-01-02"),
			"y": entry.Val,
		})
	}
	return data
}

func (s *Stats) GetStudentLiveActivityCourseStats(courseID uint, from time.Time, to time.Time) (*TimeValues, error) {
	query := fmt.Sprintf(`from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "true")
	|> group(columns: ["stream"])
	|> max()
	|> group()
	|> aggregateWindow(every: 1d, fn: median, createEmpty: false)
	|> keep(columns: ["_time", "_value"])`, from.Unix(), to.Unix(), courseID)

	res := TimeValues{}

	queryResult, err := s.query.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	for queryResult.Next() {
		res.Entries = append(res.Entries, TimeValueEntry{
			Time: queryResult.Record().Time(),
			Val:  parseValue(queryResult.Record().Value(), 0),
		})
	}

	return &res, nil
}

func (s *Stats) GetStudentVODActivityCourseStats(courseID uint, from time.Time, to time.Time) (*TimeValues, error) {
	query := fmt.Sprintf(`from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "false")
	|> group(columns: ["stream"])
	|> max()
	|> group()
	|> aggregateWindow(every: 1d, fn: median, createEmpty: false)
	|> keep(columns: ["_time", "_value"])`, from.Unix(), to.Unix(), courseID)

	res := TimeValues{}

	queryResult, err := s.query.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	for queryResult.Next() {
		res.Entries = append(res.Entries, TimeValueEntry{
			Time: queryResult.Record().Time(),
			Val:  parseValue(queryResult.Record().Value(), 0),
		})
	}

	return &res, nil
}

func parseValue(value interface{}, defaultValue int) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	default:
		return defaultValue
	}
}
