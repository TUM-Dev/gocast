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
// than one in particular. api/statistics.go's getStats/exportStats handlers use the
// same trick for the courseID == 0 case of the per-course statistics page; that page
// is unrelated to this endpoint beyond sharing the convention, and its handlers stay
// in api/ untouched.
const serverStatsCourseID = 0

// GetServerStats returns the quick counters and charted activity for the server-wide
// statistics page. Gated on server.administer by its policy in services.go.
func (a *API) GetServerStats(ctx context.Context, _ *emptypb.Empty) (*protobuf.ServerStatsResponse, error) {
	numStudents, err := a.dao.StatisticsDao.GetCourseNumStudents(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	vodViews, err := a.dao.StatisticsDao.GetCourseNumVodViews(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	liveViews, err := a.dao.StatisticsDao.GetCourseNumLiveViews(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	activityLive, err := a.dao.StatisticsDao.GetStudentActivityCourseStats(serverStatsCourseID, true)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	activityVod, err := a.dao.StatisticsDao.GetStudentActivityCourseStats(serverStatsCourseID, false)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	hourly, err := a.dao.StatisticsDao.GetCourseStatsHourly(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	weekdays, err := a.dao.StatisticsDao.GetCourseStatsWeekdays(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	allDays, err := a.dao.StatisticsDao.GetCourseNumVodViewsPerDay(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	streams, err := a.dao.StreamsDao.GetAllStreams()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.ServerStatsResponse{
		NumStudents:  numStudents,
		VodViews:     int32(vodViews),
		LiveViews:    int32(liveViews),
		NumLectures:  int32(numLectures(streams)),
		ActivityLive: serverStatsSeries("Live", activityLive),
		ActivityVod:  serverStatsSeries("VoD", activityVod),
		Hourly:       serverStatsSeries("Sum(viewers)", hourly),
		Weekdays:     serverStatsSeries("Sum(viewers)", weekdays),
		AllDays:      serverStatsSeries("views", allDays),
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
// courseID == 0 case (that handler is left in place; it still serves the per-course
// page's export).
func (a *API) ExportServerStats(ctx context.Context, req *protobuf.ExportServerStatsRequest) (*httpbody.HttpBody, error) {
	format := req.GetFormat()
	if format != "json" && format != "csv" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("format must be 'json' or 'csv'"))
	}

	weekdays, err := a.dao.StatisticsDao.GetCourseStatsWeekdays(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	hourly, err := a.dao.StatisticsDao.GetCourseStatsHourly(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	activityLive, err := a.dao.StatisticsDao.GetStudentActivityCourseStats(serverStatsCourseID, true)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	activityVod, err := a.dao.StatisticsDao.GetStudentActivityCourseStats(serverStatsCourseID, false)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	allDays, err := a.dao.StatisticsDao.GetCourseNumVodViewsPerDay(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	numStudents, err := a.dao.StatisticsDao.GetCourseNumStudents(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	vodViews, err := a.dao.StatisticsDao.GetCourseNumVodViews(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	liveViews, err := a.dao.StatisticsDao.GetCourseNumLiveViews(serverStatsCourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	result := tools.ExportStatsContainer{}
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "day", XName: "Weekday", YName: "Sum(viewers)", Data: weekdays})
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "hour", XName: "Hour", YName: "Sum(viewers)", Data: hourly})
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "activity-live", XName: "Week", YName: "Live", Data: activityLive})
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "activity-vod", XName: "Week", YName: "VoD", Data: activityVod})
	result = result.AddDataEntry(&tools.ExportDataEntry{Name: "allDays", XName: "Week", YName: "VoD", Data: allDays})
	result = result.AddDataEntry(&tools.ExportDataEntry{
		Name:  "quickStats",
		XName: "Property",
		YName: "Value",
		Data: []dao.Stat{
			{X: "Enrolled Students", Y: int(numStudents)},
			{X: "Vod Views", Y: vodViews},
			{X: "Live Views", Y: liveViews},
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
