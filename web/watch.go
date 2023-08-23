package web

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/api"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

func (r mainRoutes) WatchPage(c *gin.Context) {
	span := sentry.StartSpan(c, "GET /w", sentry.TransactionName("GET /w"))
	defer span.Finish()
	var data WatchPageData
	err := data.Prepare(c, r.LectureHallsDao)
	if err != nil {
		log.WithError(err).Error("Can't prepare data for watch page")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	data.IndexData = NewIndexData()
	if tumLiveContext.Course.DownloadsEnabled && tumLiveContext.Stream.IsDownloadable() {
		err = tools.SetSignedPlaylists(tumLiveContext.Stream, tumLiveContext.User, true)
	} else {
		err = tools.SetSignedPlaylists(tumLiveContext.Stream, tumLiveContext.User, false)
	}
	if err != nil {
		log.WithError(err).Warn("Can't sign playlists")
	}
	data.IndexData.TUMLiveContext = tumLiveContext
	data.IsAdminOfCourse = tumLiveContext.UserIsAdmin()
	data.AlertsEnabled = tools.Cfg.Alerts != nil

	data.ChatData.IndexData.TUMLiveContext = foundContext.(tools.TUMLiveContext)
	data.ChatData.IsAdminOfCourse = tumLiveContext.UserIsAdmin()

	if data.IsAdminOfCourse && tumLiveContext.Stream.LectureHallID != 0 {
		lectureHall, err := r.LectureHallsDao.GetLectureHallByID(tumLiveContext.Stream.LectureHallID)
		if err != nil {
			sentry.CaptureException(err)
		} else {
			data.Presets = lectureHall.CameraPresets
		}
	}

	if c.Param("version") != "" {
		data.Version = c.Param("version")
		if strings.HasPrefix(data.Version, "unit-") {
			if unitID, err := strconv.Atoi(strings.ReplaceAll(data.Version, "unit-", "")); err == nil && unitID < len(tumLiveContext.Stream.Units) {
				data.Unit = &tumLiveContext.Stream.Units[unitID]
			}
		}
	}

	if tumLiveContext.Stream.LectureHallID != 0 {
		switch data.IndexData.TUMLiveContext.Course.GetSourceModeForLectureHall(data.IndexData.TUMLiveContext.Stream.LectureHallID) {
		// SourceMode == 1 -> Override Version to PRES
		case 1:
			data.Version = "PRES"
			data.IndexData.TUMLiveContext.Stream.PlaylistUrlCAM = ""
			data.IndexData.TUMLiveContext.Stream.PlaylistUrl = ""
		// SourceMode == 2 -> Override Version to CAM
		case 2:
			data.Version = "CAM"
			data.IndexData.TUMLiveContext.Stream.PlaylistUrlPRES = ""
			data.IndexData.TUMLiveContext.Stream.PlaylistUrl = ""
		}
	}

	// Check for fetching progress
	if tumLiveContext.User != nil && tumLiveContext.Stream.Recording {

		progress, err := dao.Progress.LoadProgress(tumLiveContext.User.ID, []uint{tumLiveContext.Stream.ID})
		if err != nil {
			data.Progress = model.StreamProgress{Progress: 0}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				log.WithError(err).Warn("Couldn't fetch progress from the database.")
			}
		} else if len(progress) > 0 {
			data.Progress = progress[0]
		}
	}
	if c.Query("restart") == "1" {
		c.Redirect(http.StatusFound, strings.Split(c.Request.RequestURI, "?")[0])
		return
	}
	if _, dvr := c.GetQuery("dvr"); dvr {
		data.DVR = "?dvr"
	} else {
		data.DVR = ""
	}
	data.CutOffLength = api.CutOffLength
	if strings.HasPrefix(data.Version, "unit-") {
		data.Description = data.Unit.GetDescriptionHTML()
	} else {
		data.Description = template.HTML(data.IndexData.TUMLiveContext.Stream.GetDescriptionHTML())
	}
	if c.Query("video_only") == "1" {
		err := templateExecutor.ExecuteTemplate(c.Writer, "video_only.gohtml", data)
		if err != nil {
			log.Printf("couldn't render template: %v\n", err)
		}
	} else {
		err := templateExecutor.ExecuteTemplate(c.Writer, "watch.gohtml", data)
		if err != nil {
			log.Printf("couldn't render template: %v\n", err)
		}
	}
}

// WatchPageData contains all the metadata that is related to the watch page.
type WatchPageData struct {
	IsAdminOfCourse bool // is current user admin or lecturer who created this course
	IsHighlightPage bool
	AlertsEnabled   bool // whether the alert config is set
	Version         string
	Unit            *model.StreamUnit
	Presets         []model.CameraPreset
	Progress        model.StreamProgress
	IndexData       IndexData
	Description     template.HTML
	CutOffLength    int    // The maximum length for the preview of a description.
	DVR             string // ?dvr if dvr is enabled, empty string otherwise
	LectureHallName string
	ChatData        ChatData
}

// Prepare populates the data for the watch page.
func (d *WatchPageData) Prepare(c *gin.Context, lectureHallsDao dao.LectureHallsDao) error {
	// todo prepare rest of data here as well
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		return errors.New("context should exist but doesn't")
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	err := d.prepareLectureHall(tumLiveContext, lectureHallsDao)
	if err != nil {
		return err
	}
	return nil
}

func (d *WatchPageData) prepareLectureHall(c tools.TUMLiveContext, lectureHallsDao dao.LectureHallsDao) error {
	if c.Stream.LectureHallID != 0 {
		lectureHall, err := lectureHallsDao.GetLectureHallByID(c.Stream.LectureHallID)
		if err != nil {
			return err
		}
		d.LectureHallName = lectureHall.Name
	}
	return nil
}
