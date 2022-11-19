package stats

import (
	"context"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"github.com/joschahenningsen/TUM-Live/model"
	"strconv"
	"time"
)

type Stats struct {
	bucket    string
	client    influxdb2.Client
	liveStats api.WriteAPI
	query     api.QueryAPI
}

var weekdays = []string{
	"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday",
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

// GetStreamNumLiveViews returns the latest data of live viewers
func (s *Stats) GetStreamNumLiveViews(streamId uint, from time.Time, to time.Time) (int, error) {
	query := fmt.Sprintf(`from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.stream == "%d" and r.live == "true") 
    |> group()
	|> keep(columns: ["_value"])
	|> last()`, from.Unix(), to.Unix(), streamId)

	if res, err := s.query.Query(context.Background(), query); err != nil {
		return 0, err
	} else if hasRecord := res.Next(); !hasRecord {
		return 0, nil
	} else {
		return parseValueInt(res.Record().Value(), 0), nil
	}
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
	} else if hasRecord := res.Next(); !hasRecord {
		return 0, nil
	} else {
		return parseValueInt(res.Record().Value(), 0), nil
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
	} else if hasRecord := res.Next(); !hasRecord {
		return 0, nil
	} else {
		return parseValueInt(res.Record().Value(), 0), nil
	}
}

type ChartDataEntry struct {
	X string
	Y int
}

type ChartData struct {
	Entries []ChartDataEntry
}

func (t *ChartData) GetChartJsData() []map[string]any {
	var data = make([]map[string]any, 0)
	for _, entry := range t.Entries {
		data = append(data, map[string]any{
			"x": entry.X,
			"y": entry.Y,
		})
	}
	return data
}

func (s *Stats) GetStudentLiveActivityCourseStats(courseID uint, from time.Time, to time.Time) (*ChartData, error) {
	query := fmt.Sprintf(`from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "true")
	|> group(columns: ["stream"])
	|> max()
	|> group()
	|> aggregateWindow(every: 7d, fn: median, createEmpty: false)
	|> keep(columns: ["_time", "_value"])`, from.Unix(), to.Unix(), courseID)

	res := ChartData{}

	queryResult, err := s.query.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	for queryResult.Next() {
		res.Entries = append(res.Entries, ChartDataEntry{
			X: queryResult.Record().Time().Format("2006-01-02"),
			Y: parseValueInt(queryResult.Record().Value(), 0),
		})
	}

	return &res, nil
}

func (s *Stats) GetStudentVODActivityCourseStats(courseID uint, from time.Time, to time.Time) (*ChartData, error) {
	query := fmt.Sprintf(`from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "false")
	|> group(columns: ["stream"])
	|> max()
	|> group()
	|> aggregateWindow(every: 7d, fn: median, createEmpty: false)
	|> keep(columns: ["_time", "_value"])`, from.Unix(), to.Unix(), courseID)

	res := ChartData{}

	queryResult, err := s.query.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	for queryResult.Next() {
		res.Entries = append(res.Entries, ChartDataEntry{
			X: queryResult.Record().Time().Format("2006-01-02"),
			Y: parseValueInt(queryResult.Record().Value(), 0),
		})
	}

	return &res, nil
}

func (s *Stats) GetCourseStatsHourly(courseID uint, from time.Time, to time.Time) (*ChartData, error) {
	query := fmt.Sprintf(`import "date"
	from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "false")
    |> map(fn: (r) => ({ r with hour: date.hour(t: r._time) }))  
    |> group(columns: ["hour"], mode:"by")
    |> sum()
    |> group()
	`, from.Unix(), to.Unix(), courseID)

	res := ChartData{}

	queryResult, err := s.query.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	for queryResult.Next() {
		hour := strconv.Itoa(parseValueInt(queryResult.Record().ValueByKey("hour"), 0))

		res.Entries = append(res.Entries, ChartDataEntry{
			X: hour,
			Y: parseValueInt(queryResult.Record().Value(), 0),
		})
	}

	return &res, nil
}

func (s *Stats) GetCourseStatsWeekday(courseID uint, from time.Time, to time.Time) (*ChartData, error) {
	query := fmt.Sprintf(`import "date"
	from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "false")
    |> map(fn: (r) => ({ r with day: date.weekDay(t: r._time) }))  
    |> group(columns: ["day"], mode:"by")
    |> sum()
    |> group()
	`, from.Unix(), to.Unix(), courseID)

	res := ChartData{}

	queryResult, err := s.query.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	for queryResult.Next() {
		hour := weekdays[parseValueInt(queryResult.Record().ValueByKey("day"), 0)]

		res.Entries = append(res.Entries, ChartDataEntry{
			X: hour,
			Y: parseValueInt(queryResult.Record().Value(), 0),
		})
	}

	return &res, nil
}

func (s *Stats) GetStudentVODPerDay(courseID uint, from time.Time, to time.Time) (*ChartData, error) {
	query := fmt.Sprintf(`from(bucket: "live_stats")
	|> range(start: %d, stop: %d)
	|> filter(fn: (r) => r.course == "%d" and r.live == "false")
    |> window(every: 1d)
    |> sum()
    |> group()
	|> keep(columns: ["_start", "_value"])`, from.Unix(), to.Unix(), courseID)

	res := ChartData{}

	queryResult, err := s.query.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	for queryResult.Next() {
		res.Entries = append(res.Entries, ChartDataEntry{
			X: queryResult.Record().Start().Format("2006-01-02"),
			Y: parseValueInt(queryResult.Record().Value(), 0),
		})
	}

	return &res, nil
}

// / parseValueInt parses a value to int, if not able to parse returns the defaultValue
func parseValueInt(value interface{}, defaultValue int) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int64:
		return int(v)
	case string:
		if num, err := strconv.Atoi(v); err != nil {
			return defaultValue
		} else {
			return num
		}
	default:
		return defaultValue
	}
}
