package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
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
		res, err := r.StatisticsDao.GetCourseStatsWeekdays(cid)
		if err != nil {
			logger.Warn("GetCourseStatsWeekdays failed", "err", err, "courseId", cid)
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
		resp.Data.Datasets[0].Data = res
		c.JSON(http.StatusOK, resp)
	case "hour":
		res, err := r.StatisticsDao.GetCourseStatsHourly(cid)
		if err != nil {
			logger.Warn("GetCourseStatsHourly failed", "err", err, "courseId", cid)
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
		resp.Data.Datasets[0].Data = res
		c.JSON(http.StatusOK, resp)
	case "activity-live":
		resLive, err := r.StatisticsDao.GetStudentActivityCourseStats(cid, true)
		if err != nil {
			logger.Warn("GetCourseStatsLive failed", "err", err, "courseId", cid)
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
		resp.Data.Datasets[0].Data = resLive
		resp.Data.Datasets[0].BorderColor = "#d12a5c"
		resp.Data.Datasets[0].BackgroundColor = ""

		c.JSON(http.StatusOK, resp)
	case "activity-vod":
		resVod, err := r.StatisticsDao.GetStudentActivityCourseStats(cid, false)
		if err != nil {
			logger.Warn("GetCourseStatsVod failed", "err", err, "courseId", cid)
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
		resp.Data.Datasets[0].Data = resVod
		resp.Data.Datasets[0].BorderColor = "#2a7dd1"
		resp.Data.Datasets[0].BackgroundColor = ""
		c.JSON(http.StatusOK, resp)
	case "numStudents":
		res, err := r.StatisticsDao.GetCourseNumStudents(cid)
		if err != nil {
			logger.Warn("GetCourseNumStudents failed", "err", err, "courseId", cid)
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
		res, err := r.StatisticsDao.GetCourseNumVodViews(cid)
		if err != nil {
			logger.Warn("GetCourseNumVodViews failed", "err", err, "courseId", cid)
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
		res, err := r.StatisticsDao.GetCourseNumLiveViews(cid)
		if err != nil {
			logger.Warn("GetCourseNumLiveViews failed", "err", err, "courseId", cid)
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
			res, err := r.StatisticsDao.GetCourseNumVodViewsPerDay(cid)
			if err != nil {
				logger.Warn("GetCourseNumLiveViews failed", "err", err, "courseId", cid)
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
				resp.Data.Datasets[0].Data = res
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
		logger.Warn("exportStats failed, invalid format", "courseId", cid)
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
			res, err := r.StatisticsDao.GetCourseStatsWeekdays(cid)
			if err != nil {
				logger.Warn("GetCourseStatsWeekdays failed", "err", err, "courseId", cid)
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Weekday",
				YName: "Sum(viewers)",
				Data:  res,
			})

		case "hour":
			res, err := r.StatisticsDao.GetCourseStatsHourly(cid)
			if err != nil {
				logger.Warn("GetCourseStatsHourly failed", "err", err, "courseId", cid)
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Hour",
				YName: "Sum(viewers)",
				Data:  res,
			})

		case "activity-live":
			resLive, err := r.StatisticsDao.GetStudentActivityCourseStats(cid, true)
			if err != nil {
				logger.Warn("GetStudentActivityCourseStats failed", "err", err, "courseId", cid)
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "Live",
				Data:  resLive,
			})

		case "activity-vod":
			resVod, err := r.StatisticsDao.GetStudentActivityCourseStats(cid, false)
			if err != nil {
				logger.Warn("GetStudentActivityCourseStats failed", "err", err, "courseId", cid)
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "VoD",
				Data:  resVod,
			})

		case "allDays":
			res, err := r.StatisticsDao.GetCourseNumVodViewsPerDay(cid)
			if err != nil {
				logger.Warn("GetCourseNumVodViewsPerDay failed", "err", err, "courseId", cid)
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "VoD",
				Data:  res,
			})

		case "quickStats":
			var quickStats []dao.Stat

			numStudents, err := r.StatisticsDao.GetCourseNumStudents(cid)
			if err != nil {
				logger.Warn("GetCourseNumStudents failed", "err", err, "courseId", cid)
			} else {
				quickStats = append(quickStats, dao.Stat{X: "Enrolled Students", Y: int(numStudents)})
			}

			vodViews, err := r.StatisticsDao.GetCourseNumVodViews(cid)
			if err != nil {
				logger.Warn("GetCourseNumVodViews failed", "err", err, "courseId", cid)
			} else {
				quickStats = append(quickStats, dao.Stat{X: "Vod Views", Y: int(vodViews)})
			}

			liveViews, err := r.StatisticsDao.GetCourseNumLiveViews(cid)
			if err != nil {
				logger.Warn("GetCourseNumLiveViews failed", "err", err, "courseId", cid)
			} else {
				quickStats = append(quickStats, dao.Stat{X: "Live Views", Y: int(liveViews)})
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Property",
				YName: "Value",
				Data:  quickStats,
			})

		default:
			logger.Warn("Invalid export interval", "courseId", cid)
		}
	}

	if req.Format == "json" {
		jsonResult, err := json.Marshal(result.ExportJson())
		if err != nil {
			logger.Warn("json.Marshal failed for stats export", "err", err, "courseId", cid)
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
