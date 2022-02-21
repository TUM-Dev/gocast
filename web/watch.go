package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

func WatchPage(c *gin.Context) {
	span := sentry.StartSpan(c, "GET /w", sentry.TransactionName("GET /w"))
	defer span.Finish()
	var data WatchPageData
	err := data.Prepare(c)
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
	data.IndexData.TUMLiveContext = tumLiveContext
	data.IsAdminOfCourse = tumLiveContext.UserIsAdmin()

	data.ChatData.IndexData.TUMLiveContext = foundContext.(tools.TUMLiveContext)
	data.ChatData.IsAdminOfCourse = tumLiveContext.UserIsAdmin()
	data.ChatData.IsPopUp = false

	if data.IsAdminOfCourse && tumLiveContext.Stream.LectureHallID != 0 {
		lectureHall, err := dao.GetLectureHallByID(tumLiveContext.Stream.LectureHallID)
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
	// Check for fetching progress
	if tumLiveContext.User != nil && tumLiveContext.Stream.Recording {
		progress, err := dao.LoadProgress(tumLiveContext.User.ID, tumLiveContext.Stream.ID)
		if err != nil {
			data.Progress = model.StreamProgress{Progress: 0}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				log.WithError(err).Warn("Couldn't fetch progress from the database.")
			}
		} else {
			data.Progress = progress
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

	if strings.HasPrefix(data.Version, "unit-") {
		data.Description = template.HTML(data.Unit.GetDescriptionHTML())
	} else {
		data.Description = template.HTML(data.IndexData.TUMLiveContext.Stream.GetDescriptionHTML())
	}
	if c.Query("video_only") == "1" {
		err := templ.ExecuteTemplate(c.Writer, "video_only.gohtml", data)
		if err != nil {
			log.Printf("couldn't render template: %v\n", err)
		}
	} else {
		err := templ.ExecuteTemplate(c.Writer, "watch.gohtml", data)
		if err != nil {
			log.Printf("couldn't render template: %v\n", err)
		}
	}
}

// WatchPageData contains all the metadata that is related to the watch page.
type WatchPageData struct {
	IsAdminOfCourse bool // is current user admin or lecturer who created this course
	IsHighlightPage bool
	Version         string
	Unit            *model.StreamUnit
	Presets         []model.CameraPreset
	Progress        model.StreamProgress
	IndexData       IndexData
	Description     template.HTML
	DVR             string // ?dvr if dvr is enabled, empty string otherwise
	LectureHallName string
	ChatData		ChatData
}

// Prepare populates the data for the watch page.
func (d *WatchPageData) Prepare(c *gin.Context) error {
	// todo prepare rest of data here as well
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		return errors.New("context should exist but doesn't")
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	err := d.prepareLectureHall(tumLiveContext)
	if err != nil {
		return err
	}
	return nil
}

func (d *WatchPageData) prepareLectureHall(c tools.TUMLiveContext) error {
	if c.Stream.LectureHallID != 0 {
		lectureHall, err := dao.GetLectureHallByID(c.Stream.LectureHallID)
		if err != nil {
			return err
		}
		d.LectureHallName = lectureHall.Name
	}
	return nil
}
