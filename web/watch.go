package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func WatchPage(c *gin.Context) {
	log.Printf("watchpage")
	span := sentry.StartSpan(c, "GET /w", sentry.TransactionName("GET /w"))
	defer span.Finish()
	var data WatchPageData
	user, userErr := tools.GetUser(c)
	student, studentErr := tools.GetStudent(c)
	data.IndexData = NewIndexData()
	if userErr == nil {
		data.IndexData.IsUser = true
		data.IndexData.IsAdmin = user.Role == model.AdminType || user.Role == model.LecturerType
	}
	if studentErr == nil {
		data.IndexData.IsStudent = true
	}
	vodID := c.Param("id")
	vod, err := dao.GetStreamByID(context.Background(), vodID)
	if err != nil {
		c.Status(http.StatusNotFound)
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: data.IndexData, Status:http.StatusNotFound,Message: "Couldn't find stream."})
		return
	}
	data.Stream = vod
	if c.Param("version") != "" {
		data.Version = c.Param("version")
		if strings.HasPrefix(data.Version, "unit-") {
			if unitID, err := strconv.Atoi(strings.ReplaceAll(data.Version, "unit-", "")); err == nil && unitID < len(vod.Units) {
				data.Unit = &vod.Units[unitID]
			}
		}
	}
	course, err := dao.GetCourseById(context.Background(), vod.CourseID)
	if err != nil {
		log.Printf("couldn't find course for stream: %v\n", err)
		c.Status(http.StatusNotFound)
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: data.IndexData, Status:http.StatusNotFound,Message: "Couldn't find stream."})
		return
	}
	if course.Visibility == "loggedin" && userErr != nil && studentErr != nil {
		c.Status(http.StatusForbidden)
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: data.IndexData, Status:http.StatusForbidden,Message: "Please log in to access this resource."})
		return
	}
	if course.Visibility == "enrolled" && !dao.IsUserAllowedToWatchPrivateCourse(course.ID, user, userErr, student, studentErr) {
		c.Status(http.StatusForbidden)
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: data.IndexData, Status:http.StatusForbidden ,Message: "You are not allowed to watch this lecture. Please log in or contact your instructor."})
		return
	}
	data.Course = course
	if strings.HasPrefix(data.Version, "unit-") {
		data.Description = template.HTML(data.Unit.GetDescriptionHTML())
	} else {
		data.Description = template.HTML(data.Stream.GetDescriptionHTML())
	}
	if c.Param("version") == "video-only" {
		err = templ.ExecuteTemplate(c.Writer, "video_only.gohtml", data)
	} else {
		err = templ.ExecuteTemplate(c.Writer, "watch.gohtml", data)
	}
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}

type WatchPageData struct {
	IndexData   IndexData
	Stream      model.Stream
	Unit        *model.StreamUnit
	Description template.HTML
	Course      model.Course
	Version     string
}
