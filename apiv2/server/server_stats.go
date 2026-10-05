package apiv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// serverStatsCourseID is 0, which dao/statistics.go treats as "every course" rather
// than one in particular. The course statistics RPCs (course_stats.go) run the same
// queries with a real ID; requiresCourseAdmin refuses course 0, so they cannot be
// used to reach the server-wide numbers.
const serverStatsCourseID = 0

// usageStats is what both statistics pages show, for one course or (with
// serverStatsCourseID) for all of them.
type usageStats struct {
	numStudents  int64
	vodViews     int
	liveViews    int
	activityLive []dao.Stat
	activityVod  []dao.Stat
	hourly       []dao.Stat
	weekdays     []dao.Stat
	allDays      []dao.Stat
}

func (a *API) collectUsageStats(courseID uint) (usageStats, error) {
	var (
		s   usageStats
		err error
	)
	if s.numStudents, err = a.dao.StatisticsDao.GetCourseNumStudents(courseID); err != nil {
		return s, e.WithStatus(http.StatusInternalServerError, err)
	}
	if s.vodViews, err = a.dao.StatisticsDao.GetCourseNumVodViews(courseID); err != nil {
		return s, e.WithStatus(http.StatusInternalServerError, err)
	}
	if s.liveViews, err = a.dao.StatisticsDao.GetCourseNumLiveViews(courseID); err != nil {
		return s, e.WithStatus(http.StatusInternalServerError, err)
	}
	if s.activityLive, err = a.dao.StatisticsDao.GetStudentActivityCourseStats(courseID, true); err != nil {
		return s, e.WithStatus(http.StatusInternalServerError, err)
	}
	if s.activityVod, err = a.dao.StatisticsDao.GetStudentActivityCourseStats(courseID, false); err != nil {
		return s, e.WithStatus(http.StatusInternalServerError, err)
	}
	if s.hourly, err = a.dao.StatisticsDao.GetCourseStatsHourly(courseID); err != nil {
		return s, e.WithStatus(http.StatusInternalServerError, err)
	}
	if s.weekdays, err = a.dao.StatisticsDao.GetCourseStatsWeekdays(courseID); err != nil {
		return s, e.WithStatus(http.StatusInternalServerError, err)
	}
	if s.allDays, err = a.dao.StatisticsDao.GetCourseNumVodViewsPerDay(courseID); err != nil {
		return s, e.WithStatus(http.StatusInternalServerError, err)
	}
	return s, nil
}

// GetServerStats returns the quick counters and charted activity for the server-wide
// statistics page. Gated on server.administer by its policy in services.go.
func (a *API) GetServerStats(ctx context.Context, _ *emptypb.Empty) (*protobuf.ServerStatsResponse, error) {
	stats, err := a.collectUsageStats(serverStatsCourseID)
	if err != nil {
		return nil, err
	}
	streams, err := a.dao.StreamsDao.GetAllStreams()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.ServerStatsResponse{
		NumStudents:  stats.numStudents,
		VodViews:     int32(stats.vodViews),
		LiveViews:    int32(stats.liveViews),
		NumLectures:  int32(numLectures(streams)),
		ActivityLive: serverStatsSeries("Live", stats.activityLive),
		ActivityVod:  serverStatsSeries("VoD", stats.activityVod),
		Hourly:       serverStatsSeries("Sum(viewers)", stats.hourly),
		Weekdays:     serverStatsSeries("Sum(viewers)", stats.weekdays),
		AllDays:      serverStatsSeries("views", stats.allDays),
	}, nil
}

// numLectures mirrors model.Course.NumStreams(), which the old page called on a
// Course carrying every stream in the deployment for the courseID == 0 case.
func numLectures(streams []model.Stream) int {
	count := 0
	for _, stream := range streams {
		if stream.Recording || stream.LiveNow {
			count++
		}
	}
	return count
}

func serverStatsSeries(label string, stats []dao.Stat) *protobuf.ServerStatsSeries {
	points := make([]*protobuf.ServerStatsDataPoint, 0, len(stats))
	for _, stat := range stats {
		points = append(points, &protobuf.ServerStatsDataPoint{X: stat.X, Y: int32(stat.Y)})
	}
	return &protobuf.ServerStatsSeries{Label: label, Points: points}
}

// ExportServerStats bundles the same data GetServerStats renders into charts as a
// single downloadable file, mirroring api/statistics.go's exportStats for its
// courseID == 0 case.
func (a *API) ExportServerStats(ctx context.Context, req *protobuf.ExportServerStatsRequest) (*httpbody.HttpBody, error) {
	return a.exportUsageStats(serverStatsCourseID, req.GetFormat())
}

// exportUsageStats renders usageStats as the file v1's exportStats produced, with the
// same entry names, so a spreadsheet built on the old export keeps working.
func (a *API) exportUsageStats(courseID uint, format string) (*httpbody.HttpBody, error) {
	if format != "json" && format != "csv" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("format must be 'json' or 'csv'"))
	}

	stats, err := a.collectUsageStats(courseID)
	if err != nil {
		return nil, err
	}

	result := tools.ExportStatsContainer{}
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "day", XName: "Weekday", YName: "Sum(viewers)", Data: stats.weekdays})
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "hour", XName: "Hour", YName: "Sum(viewers)", Data: stats.hourly})
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "activity-live", XName: "Week", YName: "Live", Data: stats.activityLive})
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "activity-vod", XName: "Week", YName: "VoD", Data: stats.activityVod})
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "allDays", XName: "Week", YName: "VoD", Data: stats.allDays})
	result = result.AddDataEntry(&tools.ExportDataEntry{
		Name:  "quickStats",
		XName: "Property",
		YName: "Value",
		Data: []dao.Stat{
			{X: "Enrolled Students", Y: int(stats.numStudents)},
			{X: "Vod Views", Y: stats.vodViews},
			{X: "Live Views", Y: stats.liveViews},
		},
	})

	if format == "json" {
		data, err := json.Marshal(result.ExportJson())
		if err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		return &httpbody.HttpBody{ContentType: "application/octet-stream", Data: data}, nil
	}

	return &httpbody.HttpBody{ContentType: "application/octet-stream", Data: []byte(result.ExportCsv())}, nil
}
