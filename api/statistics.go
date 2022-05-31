package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
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
	if c.ShouldBindQuery(&req) != nil {
		c.AbortWithStatus(http.StatusBadRequest)
	}
	var cid uint
	// check if request is for server -> validate
	cidFromContext := c.Param("courseId")
	if cidFromContext == "0" {
		if ctx.(tools.TUMLiveContext).User.Role != model.AdminType {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		cid = 0
	} else { // use course from context
		cid = ctx.(tools.TUMLiveContext).Course.ID
	}
	switch req.Interval {
	case "week":
	case "day":
		res, err := r.StatisticsDao.GetCourseStatsWeekdays(cid)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsWeekdays failed")
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
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsHourly failed")
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
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsLive failed")
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
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsVod failed")
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
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumStudents failed")
			c.AbortWithStatus(http.StatusInternalServerError)
		} else {
			c.JSON(http.StatusOK, gin.H{"res": res})
		}
	case "vodViews":
		res, err := r.StatisticsDao.GetCourseNumVodViews(cid)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumVodViews failed")
			c.AbortWithStatus(http.StatusInternalServerError)
		} else {
			c.JSON(http.StatusOK, gin.H{"res": res})
		}
	case "liveViews":
		res, err := r.StatisticsDao.GetCourseNumLiveViews(cid)
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumLiveViews failed")
			c.AbortWithStatus(http.StatusInternalServerError)
		} else {
			c.JSON(http.StatusOK, gin.H{"res": res})
		}
	case "allDays":
		{
			res, err := r.StatisticsDao.GetCourseNumVodViewsPerDay(cid)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumLiveViews failed")
				c.AbortWithStatus(http.StatusInternalServerError)
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
		c.AbortWithStatus(http.StatusBadRequest)
	}
}

func (r coursesRoutes) exportStats(c *gin.Context) {
	ctx, _ := c.Get("TUMLiveContext")

	var req statExportReq
	if c.ShouldBindQuery(&req) != nil {
		c.AbortWithStatus(http.StatusBadRequest)
	}

	var cid uint
	// check if request is for server -> validate
	cidFromContext := c.Param("courseId")
	if cidFromContext == "0" {
		if ctx.(tools.TUMLiveContext).User.Role != model.AdminType {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		cid = 0
	} else { // use course from context
		cid = ctx.(tools.TUMLiveContext).Course.ID
	}

	if req.Format != "json" && req.Format != "csv" {
		log.WithField("courseId", cid).Warn("exportStats failed, invalid format")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	result := tools.ExportStatsContainer{}

	for _, interval := range req.Interval {
		switch interval {
		case "week":
		case "day":
			res, err := r.StatisticsDao.GetCourseStatsWeekdays(cid)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsWeekdays failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Weekday",
				YName: "Sum(viewers)",
				Data:  res,
			})
			break

		case "hour":
			res, err := r.StatisticsDao.GetCourseStatsHourly(cid)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseStatsHourly failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Hour",
				YName: "Sum(viewers)",
				Data:  res,
			})
			break

		case "activity-live":
			resLive, err := r.StatisticsDao.GetStudentActivityCourseStats(cid, true)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetStudentActivityCourseStats failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "Live",
				Data:  resLive,
			})
			break

		case "activity-vod":
			resVod, err := r.StatisticsDao.GetStudentActivityCourseStats(cid, false)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetStudentActivityCourseStats failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "VoD",
				Data:  resVod,
			})
			break

		case "allDays":
			res, err := r.StatisticsDao.GetCourseNumVodViewsPerDay(cid)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumVodViewsPerDay failed")
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Week",
				YName: "VoD",
				Data:  res,
			})
			break

		case "quickStats":
			var quickStats []dao.Stat

			numStudents, err := r.StatisticsDao.GetCourseNumStudents(cid)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumStudents failed")
			} else {
				quickStats = append(quickStats, dao.Stat{X: "Enrolled Students", Y: int(numStudents)})
			}

			vodViews, err := r.StatisticsDao.GetCourseNumVodViews(cid)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumVodViews failed")
			} else {
				quickStats = append(quickStats, dao.Stat{X: "Vod Views", Y: int(vodViews)})
			}

			liveViews, err := r.StatisticsDao.GetCourseNumLiveViews(cid)
			if err != nil {
				log.WithError(err).WithField("courseId", cid).Warn("GetCourseNumLiveViews failed")
			} else {
				quickStats = append(quickStats, dao.Stat{X: "Live Views", Y: int(liveViews)})
			}
			result = result.AddDataEntry(&tools.ExportDataEntry{
				Name:  interval,
				XName: "Property",
				YName: "Value",
				Data:  quickStats,
			})

			break

		default:
			log.WithField("courseId", cid).Warn("Invalid export interval")
		}
	}

	if req.Format == "json" {
		jsonResult, err := json.Marshal(result.ExportJson())
		if err != nil {
			log.WithError(err).WithField("courseId", cid).Warn("json.Marshal failed for stats export")
			c.AbortWithStatus(http.StatusInternalServerError)
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

//chartJsDataset is a single dataset ready to be used in a Chart.js chart
type chartJsDataset struct {
	Label           string         `json:"label"`
	Fill            bool           `json:"fill"`
	BorderColor     string         `json:"borderColor,omitempty"`
	BackgroundColor string         `json:"backgroundColor,omitempty"`
	Data            interface{}    `json:"data"` // whatever data
	Options         chartJsOptions `json:"options"`
}

//New creates a chartJsDataset with some defaults
func newChartJsDataset() chartJsDataset {
	return chartJsDataset{
		Fill:            false,
		BorderColor:     "#427dbd",
		BackgroundColor: "#427dbd",
	}
}
