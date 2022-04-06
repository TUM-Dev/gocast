package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"bytes"
	"errors"
	"fmt"
	campusonline "github.com/RBG-TUM/CAMPUSOnline"
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

func postSchedule(c *gin.Context) {
	resp := ""
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
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
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	yearStr := c.Param("year")
	year, err := strconv.Atoi(yearStr)
	term := c.Param("term")
	if err != nil || !(term == "W" || term == "S") {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err != nil {
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
			LiveEnabled:         false,
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
			lectureHall, err := dao.GetLectureHallByPartialName(event.RoomName)
			if err != nil {
				log.WithError(err).Error("No room found for request")
				continue
			}
			streams = append(streams, model.Stream{
				Start:         event.Start,
				End:           event.End,
				RoomName:      event.RoomName,
				LectureHallID: lectureHall.ID,
				StreamKey:     strings.ReplaceAll(uuid.NewV4().String(), "-", "")[:15],
			})
		}
		course.Streams = streams
		err := dao.CreateCourse(c, &course, !req.OptIn)
		if err != nil {
			resp += err.Error()
			continue
		}
		var users []*model.User
		for _, contact := range courseReq.Contacts {
			if !contact.MainContact {
				continue
			}
			name := contact.FirstName + " " + contact.LastName
			mail := contact.Email
			user, err := tum.FindUserWithEmail(mail)
			if err != nil || user == nil {
				log.WithError(err).Errorf("can't find user %v", mail)
				continue
			}
			time.Sleep(time.Millisecond * 200) // wait a bit, otherwise ldap locks us out
			user.Name = name
			user.Role = model.LecturerType
			err = dao.UpsertUser(user)
			if err != nil {
				log.Error(err)
			} else {
				users = append(users, user)
			}
		}
		for _, user := range users {
			if err := dao.AddAdminToCourse(user.ID, course.ID); err != nil {
				log.WithError(err).Error("can't add admin to course")
			}
			err := notifyCourseCreated(MailTmpl{
				Name:   user.Name,
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

func notifyCourseCreated(d MailTmpl, mailAddr string, subject string) error {
	templ, err := template.ParseFS(staticFS, "template/*.gotemplate")
	if err != nil {
		return err
	}
	var body bytes.Buffer
	err = templ.ExecuteTemplate(&body, "mail-course-registered.gotemplate", d)
	if err != nil {
		log.Error(err)
	}
	return tools.SendMail(tools.Cfg.Mail.Server, tools.Cfg.Mail.Sender, subject, body.String(), []string{mailAddr})
}

func getSchedule(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	rng := strings.Split(c.Request.Form.Get("range"), " to ")
	if len(rng) != 2 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	from, err := time.Parse("2006-01-02", rng[0])
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	to, err := time.Parse("2006-01-02", rng[1])
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	//todo figure out right token
	campus, err := campusonline.New(tools.Cfg.Campus.Tokens[1], "")
	if err != nil {
		log.WithError(err).Error("Can't create campus client")
		return
	}
	var room campusonline.ICalendar
	switch c.Request.Form.Get("department") {
	case "In":
		room, err = campus.GetXCalIn(from, to)
	case "Ma":
		room, err = campus.GetXCalMa(from, to)
	case "Ph":
		room, err = campus.GetXCalPh(from, to)
	default:
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		log.WithError(err).Error("Can't get room schedule")
		return
	}
	ical := &room
	ical.Filter()
	ical.Sort()
	courses := ical.GroupByCourse()
	courses, err = campus.LoadCourseContacts(courses)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
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
