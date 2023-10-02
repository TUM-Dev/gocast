package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	campusonline "github.com/RBG-TUM/CAMPUSOnline"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (r lectureHallRoutes) postSchedule(c *gin.Context) {
	resp := ""
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	type importReq struct {
		Courses []campusonline.Course `json:"courses"`
		OptIn   bool                  `json:"optIn"`
	}
	var req importReq
	err := c.BindJSON(&req)
	if err != nil {
		sentry.CaptureException(errors.New("could not bind JSON request"))
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not bind JSON request",
		})
		return
	}
	yearStr := c.Param("year")
	year, err := strconv.Atoi(yearStr)
	term := c.Param("term")
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid year",
			Err:           err,
		})
		return
	}
	if !(term == "W" || term == "S") {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid term",
		})
		return
	}
	for _, courseReq := range req.Courses {
		if !courseReq.Import {
			continue
		}
		token := strings.ReplaceAll(uuid.NewV4().String(), "-", "")[:15]
		course := model.Course{
			UserID:              tumLiveContext.User.ID,
			Name:                courseReq.Title,
			Slug:                courseReq.Slug,
			Year:                year,
			TeachingTerm:        term,
			TUMOnlineIdentifier: fmt.Sprintf("%d", courseReq.CourseID),
			VODEnabled:          false,
			DownloadsEnabled:    false,
			ChatEnabled:         false,
			Visibility:          "loggedin",
			Streams:             nil,
			Users:               nil,
			Token:               token,
		}

		var streams []model.Stream
		for _, event := range courseReq.Events {
			lectureHall, err := r.LectureHallsDao.GetLectureHallByPartialName(event.RoomName)
			if err != nil {
				log.WithError(err).Error("No room found for request")
				continue
			}
			var eventID uint
			eventIDInt, err := strconv.Atoi(event.EventID)
			if err == nil {
				eventID = uint(eventIDInt)
			}
			streams = append(streams, model.Stream{
				Start:            event.Start,
				End:              event.End,
				RoomName:         event.RoomName,
				LectureHallID:    lectureHall.ID,
				StreamKey:        strings.ReplaceAll(uuid.NewV4().String(), "-", "")[:15],
				TUMOnlineEventID: eventID,
			})
		}
		course.Streams = streams
		err := r.CoursesDao.CreateCourse(c, &course, !req.OptIn)
		if err != nil {
			resp += err.Error()
			continue
		}
		var users []*model.User
		for _, contact := range courseReq.Contacts {
			if !contact.MainContact {
				continue
			}
			mail := contact.Email
			user, err := tum.FindUserWithEmail(mail)
			if err != nil || user == nil {
				log.WithError(err).Errorf("can't find user %v", mail)
				continue
			}
			time.Sleep(time.Millisecond * 200) // wait a bit, otherwise ldap locks us out
			user.Role = model.LecturerType
			err = r.UsersDao.UpsertUser(user)
			if err != nil {
				log.Error(err)
			} else {
				users = append(users, user)
			}
		}
		for _, user := range users {
			if err := r.CoursesDao.AddAdminToCourse(user.ID, course.ID); err != nil {
				log.WithError(err).Error("can't add admin to course")
			}
			err := r.notifyCourseCreated(MailTmpl{
				Name:   user.DisplayName,
				Course: course,
				Users:  users,
				OptIn:  req.OptIn,
			}, user.Email.String, fmt.Sprintf("Vorlesungsstreams %s | Lecture streaming %s", course.Name, course.Name))
			if err != nil {
				log.WithFields(log.Fields{"course": course.Name, "email": user.Email.String}).WithError(err).Error("can't send email")
			}
			time.Sleep(time.Millisecond * 100) // 1/10th second delay, being nice to our mailrelay
		}

	}
	if resp != "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, resp)
	}
}

type MailTmpl struct {
	Name   string
	OptIn  bool
	Course model.Course
	Users  []*model.User
}

func (r lectureHallRoutes) notifyCourseCreated(d MailTmpl, mailAddr string, subject string) error {
	templ, err := template.ParseFS(staticFS, "template/*.gotemplate")
	if err != nil {
		return err
	}
	var body bytes.Buffer
	err = templ.ExecuteTemplate(&body, "mail-course-registered.gotemplate", d)
	if err != nil {
		log.Error(err)
	}
	return r.EmailDao.Create(context.Background(), &model.Email{
		From:    tools.Cfg.Mail.Sender,
		To:      mailAddr,
		Subject: subject,
		Body:    body.String(),
	})
}

func (r lectureHallRoutes) getSchedule(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not parse form",
			Err:           err,
		})
		return
	}
	rng := strings.Split(c.Request.Form.Get("range"), " to ")
	if len(rng) != 2 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid range parameter",
		})
		return
	}
	from, err := time.Parse("2006-01-02", rng[0])
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid 'from'",
		})
		return
	}
	to, err := time.Parse("2006-01-02", rng[1])
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid 'to'",
		})
		return
	}
	//todo figure out right token
	campus, err := campusonline.New(tools.Cfg.Campus.Tokens[0], "")
	if err != nil {
		log.WithError(err).Error("Can't create campus client")
		return
	}
	var room campusonline.ICalendar
	dep := c.Request.Form.Get("department")
	switch dep {
	case "Computer Science":
		room, err = campus.GetXCalCs(from, to)
	case "Computer Engineering":
		room, err = campus.GetXCalCe(from, to)
	case "Mathematics":
		room, err = campus.GetXCalMa(from, to)
	case "Physics":
		room, err = campus.GetXCalPh(from, to)
	default:
		depInt, convErr := strconv.Atoi(c.Request.Form.Get("departmentID"))
		if convErr != nil {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "invalid department",
			})
			return
		}
		room, err = campus.GetXCalOrg(from, to, depInt)
	}
	if err != nil {
		log.WithError(err).Error("can not get room schedule")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get room schedule",
			Err:           err,
		})
		return
	}
	ical := &room
	ical.Filter()
	ical.Sort()
	courses := ical.GroupByCourse()
	courses, err = campus.LoadCourseContacts(courses)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not load course contacts",
			Err:           err,
		})
		return
	}
	for _, crs := range courses {
		courseSlug := ""
		for _, l := range strings.Split(crs.Title, " ") {
			runes := []rune(l)
			if len(runes) != 0 && (unicode.IsNumber(runes[0]) || unicode.IsLetter(runes[0])) {
				courseSlug += string(runes[0])
			}
		}
	}
	c.JSON(http.StatusOK, courses)
}
