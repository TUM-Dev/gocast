package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/stats"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"time"
)

type statReq struct {
	Interval string `form:"interval" json:"interval" xml:"interval"  binding:"required"`
}

type statExportReq struct {
	Format   string   `form:"format" binding:"required"`
	Interval []string `form:"interval[]"  binding:"required"`
}

func (r coursesRoutes) getStats(c *gin.Context) {
	ctx, _ := c.Get("TUMLiveContext")

	from := time.Now().AddDate(-1, 0, 0)
	to := time.Now()

	var req statReq
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind query",
			Err:           err,
		})
		return
	}
	var cid uint
	// check if request is for server -> validate
	cidFromContext := c.Param("courseID")
	if cidFromContext == "0" {
		if ctx.(tools.TUMLiveContext).User.Role != model.AdminType {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusForbidden,
				CustomMessage: "not admin",
			})
			return
		}
		cid = 0
	} else { // use course from context
		cid = ctx.(tools.TUMLiveContext).Course.ID
	}
	switch req.Interval {
	case "week":
		fallthrough
	case "day":
		statsData, err := stats.Client.GetCourseStatsWeekday(cid, from, to)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsWeekdays failed")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not get course stats weekdays",
				Err:           err,
			})
			return
		}
		resp := chartJs{
			ChartType: "bar",
			Data:      chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}},
			Options:   newChartJsOptions(),
		}
		resp.Data.Datasets[0].Label = "Sum(viewers)"
		resp.Data.Datasets[0].Data = statsData.GetChartJsData()
		c.JSON(http.StatusOK, resp)
	case "hour":
		statsData, err := stats.Client.GetCourseStatsHourly(cid, from, to)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsHourly failed")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not get course stats hourly",
				Err:           err,
			})
			return
		}
		resp := chartJs{
			ChartType: "bar",
			Data:      chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}},
			Options:   newChartJsOptions(),
		}
		resp.Data.Datasets[0].Label = "Sum(viewers)"
		resp.Data.Datasets[0].Data = statsData.GetChartJsData()
		c.JSON(http.StatusOK, resp)
	case "activity-live":
		statsData, err := stats.Client.GetStudentLiveActivityCourseStats(cid, from, to)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsLive failed")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not get student activity course stats",
				Err:           err,
			})
			return
		}
		resp := chartJs{
			ChartType: "line",
			Data:      chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}},
			Options:   newChartJsOptions(),
		}
		resp.Data.Datasets[0].Label = "Live"
		resp.Data.Datasets[0].Data = statsData.GetChartJsData()
		resp.Data.Datasets[0].BorderColor = "#d12a5c"
		resp.Data.Datasets[0].BackgroundColor = ""

		c.JSON(http.StatusOK, resp)
	case "activity-vod":
		statsData, err := stats.Client.GetStudentVODActivityCourseStats(cid, from, to)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsVod failed")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not get student activity course stats",
				Err:           err,
			})
			return
		}
		resp := chartJs{
			ChartType: "line",
			Data:      chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}},
			Options:   newChartJsOptions(),
		}
		resp.Data.Datasets[0].Label = "VoD"
		resp.Data.Datasets[0].Data = statsData.GetChartJsData()
		resp.Data.Datasets[0].BorderColor = "#2a7dd1"
		resp.Data.Datasets[0].BackgroundColor = ""
		c.JSON(http.StatusOK, resp)
	case "numStudents":
		res, err := r.StatisticsDao.GetCourseNumStudents(cid)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumStudents failed")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not get course num students",
				Err:           err,
			})
			return
		} else {
			c.JSON(http.StatusOK, gin.H{"res": res})
		}
	case "vodViews":
		res, err := stats.Client.GetCourseNumVodViews(cid, from, to)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumVodViews failed")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not getcourse num vod views",
				Err:           err,
			})
			return
		} else {
			c.JSON(http.StatusOK, gin.H{"res": res})
		}
	case "liveViews":
		res, err := stats.Client.GetCourseNumLiveViews(cid, from, to)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumLiveViews failed")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not get course num live views",
				Err:           err,
			})
			return
		} else {
			c.JSON(http.StatusOK, gin.H{"res": res})
		}
	case "allDays":
		{
			statsData, err := stats.Client.GetStudentVODPerDay(cid, from, to)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumLiveViews failed")
				_ = c.Error(tools.RequestError{
					Status:        http.StatusInternalServerError,
					CustomMessage: "can not get course num vod views per day",
					Err:           err,
				})
				return
			} else {
				resp := chartJs{
					ChartType: "bar",
					Data:      chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}},
					Options:   newChartJsOptions(),
				}
				resp.Data.Datasets[0].Label = "views"
				resp.Data.Datasets[0].Data = statsData.GetChartJsData()
				resp.Data.Datasets[0].BackgroundColor = "#d12a5c"
				c.JSON(http.StatusOK, resp)
			}
		}
	default:
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid interval",
		})
		return
	}
}

func (r coursesRoutes) exportStats(c *gin.Context) {
	ctx, _ := c.Get("TUMLiveContext")

	from := time.Now().AddDate(-1, 0, 0)
	to := time.Now()

	var req statExportReq
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind query",
			Err:           err,
		})
		return
	}

	var cid uint
	// check if request is for server -> validate
	cidFromContext := c.Param("courseId")
	if cidFromContext == "0" {
		if ctx.(tools.TUMLiveContext).User.Role != model.AdminType {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusForbidden,
				CustomMessage: "not admin",
			})
			return
		}
		cid = 0
	} else { // use course from context
		cid = ctx.(tools.TUMLiveContext).Course.ID
	}

	if req.Format != "json" && req.Format != "csv" {
		log.WithField("courseId", cid).Warn("exportStats failed, invalid format")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "exportStats failed, invalid format",
		})
		return
	}

	result := tools.ExportStatsContainer{}

	for _, interval := range req.Interval {
		switch interval {
		case "week":
		case "day":
			statsData, err := stats.Client.GetCourseStatsWeekday(cid, from, to)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsWeekdays failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Weekday",
				YName: "Sum(viewers)",
				Data:  statsData.Entries,
			})

		case "hour":
			statsData, err := stats.Client.GetCourseStatsHourly(cid, from, to)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsHourly failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Hour",
				YName: "Sum(viewers)",
				Data:  statsData.Entries,
			})

		case "activity-live":
			statsData, err := stats.Client.GetStudentLiveActivityCourseStats(cid, from, to)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetStudentActivityCourseStats failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "Live",
				Data:  statsData.Entries,
			})

		case "activity-vod":
			statsData, err := stats.Client.GetStudentVODActivityCourseStats(cid, from, to)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetStudentActivityCourseStats failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "VoD",
				Data:  statsData.Entries,
			})

		case "allDays":
			statsData, err := stats.Client.GetStudentVODPerDay(cid, from, to)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumVodViewsPerDay failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "VoD",
				Data:  statsData.Entries,
			})

		case "quickStats":
			var quickStats []stats.ChartDataEntry

			numStudents, err := r.StatisticsDao.GetCourseNumStudents(cid)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumStudents failed")
			} else {
				quickStats = append(quickStats, stats.ChartDataEntry{X: "Enrolled Students", Y: int(numStudents)})
			}

			vodViews, err := stats.Client.GetCourseNumVodViews(cid, from, to)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumVodViews failed")
			} else {
				quickStats = append(quickStats, stats.ChartDataEntry{X: "Vod Views", Y: int(vodViews)})
			}

			liveViews, err := stats.Client.GetCourseNumLiveViews(cid, from, to)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumLiveViews failed")
			} else {
				quickStats = append(quickStats, stats.ChartDataEntry{X: "Live Views", Y: int(liveViews)})
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Property",
				YName: "Value",
				Data:  quickStats,
			})

		default:
			log.WithField("courseId", cid).Warn("Invalid export interval")
		}
	}

	if req.Format == "json" {
		jsonResult, err := json.Marshal(result.ExportJson())
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("json.Marshal failed for stats export")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "json.Marshal failed for stats export",
				Err:           err,
			})
			return
		}

		c.Header("Content-Disposition", "attachment; filename=course-"+strconv.Itoa(int(cid))+"-stats.json")
		c.Data(http.StatusOK, "application/octet-stream", jsonResult)
	} else {
		c.Header("Content-Disposition", "attachment; filename=course-"+strconv.Itoa(int(cid))+"-stats.csv")
		c.Data(http.StatusOK, "application/octet-stream", []byte(result.ExportCsv()))
	}
}

//
// Chart.js datastructures:
//

type chartJsData struct {
	Datasets []chartJsDataset `json:"datasets"`
}

type chartJs struct {
	ChartType string         `json:"type"`
	Data      chartJsData    `json:"data"`
	Options   chartJsOptions `json:"options"`
}

type chartJsScales struct {
	Y struct {
		BeginAtZero bool `json:"beginAtZero"`
	} `json:"y"`
}

func newChartJsScales() chartJsScales {
	return chartJsScales{Y: struct {
		BeginAtZero bool `json:"beginAtZero"`
	}{BeginAtZero: true}}
}

type chartJsOptions struct {
	Responsive          bool          `json:"responsive"`
	MaintainAspectRatio bool          `json:"maintainAspectRatio"`
	Scales              chartJsScales `json:"scales,omitempty"`
}

func newChartJsOptions() chartJsOptions {
	return chartJsOptions{
		Responsive:          true,
		MaintainAspectRatio: false,
		Scales:              newChartJsScales(),
	}
}

// chartJsDataset is a single dataset ready to be used in a Chart.js chart
type chartJsDataset struct {
	Label           string         `json:"label"`
	Fill            bool           `json:"fill"`
	BorderColor     string         `json:"borderColor,omitempty"`
	BackgroundColor string         `json:"backgroundColor,omitempty"`
	Data            interface{}    `json:"data"` // whatever data
	Options         chartJsOptions `json:"options"`
}

// New creates a chartJsDataset with some defaults
func newChartJsDataset() chartJsDataset {
	return chartJsDataset{
		Fill:            false,
		BorderColor:     "#427dbd",
		BackgroundColor: "#427dbd",
	}
}
