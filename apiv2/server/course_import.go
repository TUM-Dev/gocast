package apiv2

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	campusonline "github.com/RBG-TUM/CAMPUSOnline"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
)

//go:embed template
var courseImportTemplateFS embed.FS

// campusScheduleClient is the subset of campusonline.CampusOnline this handler needs,
// narrowed to an interface so tests can substitute a fake instead of reaching the real
// TUMonline. *campusonline.CampusOnline satisfies it without change.
type campusScheduleClient interface {
	GetXCalOrg(from time.Time, until time.Time, orgID int) (campusonline.ICalendar, error)
	EnrichCourse(courses []campusonline.Course) ([]campusonline.Course, error)
}

// newCampusScheduleClient builds the campus client from the configured token. A var
// so a test can point it at a fake without a real TUMonline token in config.yaml.
//
// todo figure out the right token to use rather than always the first configured one
// -- carried over unchanged from api/courseimport.go.
var newCampusScheduleClient = func() (campusScheduleClient, error) {
	if len(tools.Cfg.Campus.Tokens) == 0 {
		return nil, errors.New("no campus token configured")
	}
	return campusonline.New(tools.Cfg.Campus.Tokens[0], "")
}

// SearchCourseImportSchedule fetches the department's room schedule from TUMonline for
// the given date range, groups it by course and enriches each with its contacts and
// language, for an administrator to review before importing.
func (a *API) SearchCourseImportSchedule(ctx context.Context, req *protobuf.CourseImportSearchRequest) (*protobuf.CourseImportSearchResponse, error) {
	if req.GetFrom() == nil || req.GetTo() == nil {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("both from and to are required"))
	}
	if req.GetDepartmentId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a department id is required"))
	}

	client, err := newCampusScheduleClient()
	if err != nil {
		a.log.Error("can't create campus client", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	ical, err := client.GetXCalOrg(req.GetFrom().AsTime(), req.GetTo().AsTime(), int(req.GetDepartmentId()))
	if err != nil {
		a.log.Error("can't fetch room schedule from TUMonline", "err", err)
		return nil, e.WithStatus(http.StatusBadGateway, fmt.Errorf("fetching the schedule from TUMonline: %w", err))
	}
	ical.Filter()
	ical.Sort()

	courses, err := client.EnrichCourse(ical.GroupByCourse())
	if err != nil {
		a.log.Error("can't enrich courses with contacts from TUMonline", "err", err)
		return nil, e.WithStatus(http.StatusBadGateway, fmt.Errorf("loading course contacts from TUMonline: %w", err))
	}

	out := make([]*protobuf.CourseImportCourse, 0, len(courses))
	for _, course := range courses {
		out = append(out, courseImportCourseMessage(course))
	}
	return &protobuf.CourseImportSearchResponse{Courses: out}, nil
}

func courseImportCourseMessage(course campusonline.Course) *protobuf.CourseImportCourse {
	events := make([]*protobuf.CourseImportEvent, 0, len(course.Events))
	for _, event := range course.Events {
		events = append(events, &protobuf.CourseImportEvent{
			Start:    timestamppb.New(event.Start),
			End:      timestamppb.New(event.End),
			RoomName: event.RoomName,
			Comment:  event.Comment,
			EventId:  event.EventID,
			Import:   true,
		})
	}
	contacts := make([]*protobuf.CourseImportContact, 0, len(course.Contacts))
	for _, contact := range course.Contacts {
		contacts = append(contacts, &protobuf.CourseImportContact{
			FirstName:   contact.FirstName,
			LastName:    contact.LastName,
			Email:       contact.Email,
			Role:        contact.Role,
			MainContact: contact.MainContact,
		})
	}
	return &protobuf.CourseImportCourse{
		Title:    course.Title,
		Slug:     course.Slug,
		CourseId: int32(course.CourseID),
		Language: course.Language,
		Import:   true,
		Events:   events,
		Contacts: contacts,
	}
}

// ImportCourseImportCourses creates a course and its streams for each selected course,
// then emails its main contact an activation (or opt-out) link. A failure importing
// one course does not stop the others; the response reports each outcome so nothing
// is silently dropped, unlike the v1 handler this replaces, which concatenated errors
// into one string and either 500'd every course or reported none at all.
func (a *API) ImportCourseImportCourses(ctx context.Context, req *protobuf.CourseImportRequest) (*protobuf.CourseImportResponse, error) {
	if req.GetTerm() != "W" && req.GetTerm() != "S" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("term must be 'W' or 'S'"))
	}
	if req.GetYear() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a year is required"))
	}

	currentUser, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	results := make([]*protobuf.CourseImportResult, 0, len(req.GetCourses()))
	for _, courseReq := range req.GetCourses() {
		if !courseReq.GetImport() {
			continue
		}
		if err := a.importOneCourse(ctx, courseReq, currentUser, int(req.GetYear()), req.GetTerm(), req.GetOptIn()); err != nil {
			results = append(results, &protobuf.CourseImportResult{
				Title: courseReq.GetTitle(), Success: false, Error: err.Error(),
			})
			continue
		}
		results = append(results, &protobuf.CourseImportResult{Title: courseReq.GetTitle(), Success: true})
	}

	return &protobuf.CourseImportResponse{Results: results}, nil
}

func (a *API) importOneCourse(
	ctx context.Context,
	courseReq *protobuf.CourseImportCourse,
	currentUser *model.User,
	year int,
	term string,
	optIn bool,
) error {
	token := strings.ReplaceAll(uuid.NewV4().String(), "-", "")[:15]
	language := sql.NullString{}
	if courseReq.GetLanguage() == "de" || courseReq.GetLanguage() == "en" {
		language.Valid = true
		language.String = courseReq.GetLanguage()
	}

	course := model.Course{
		UserID:              currentUser.ID,
		Name:                courseReq.GetTitle(),
		Slug:                courseReq.GetSlug(),
		Year:                year,
		TeachingTerm:        term,
		TUMOnlineIdentifier: fmt.Sprintf("%d", courseReq.GetCourseId()),
		VODEnabled:          false,
		DownloadsEnabled:    false,
		ChatEnabled:         false,
		Visibility:          "loggedin",
		Token:               token,
		Language:            language,
	}

	var streams []model.Stream
	for _, event := range courseReq.GetEvents() {
		if !event.GetImport() {
			continue
		}
		lectureHall, err := a.dao.LectureHallsDao.GetLectureHallByPartialName(event.GetRoomName())
		if err != nil {
			a.log.Error("no room found for course import event", "room", event.GetRoomName(), "err", err)
			continue
		}
		var eventID uint
		if id, err := strconv.Atoi(event.GetEventId()); err == nil {
			eventID = uint(id)
		}
		streams = append(streams, model.Stream{
			Start:            event.GetStart().AsTime(),
			End:              event.GetEnd().AsTime(),
			RoomName:         event.GetRoomName(),
			LectureHallID:    lectureHall.ID,
			StreamKey:        strings.ReplaceAll(uuid.NewV4().String(), "-", "")[:15],
			TUMOnlineEventID: eventID,
		})
	}
	course.Streams = streams

	// !optIn: for an opt-out course, the row is created live (kept) rather than
	// soft-deleted until someone activates it -- matches CreateCourse's keep parameter
	// in the v1 handler this replaces.
	if err := a.dao.CoursesDao.CreateCourse(ctx, &course, !optIn); err != nil {
		return fmt.Errorf("creating the course: %w", err)
	}

	var admins []*model.User
	for _, contact := range courseReq.GetContacts() {
		if !contact.GetMainContact() {
			continue
		}
		user, err := tum.FindUserWithEmail(contact.GetEmail())
		if err != nil || user == nil {
			a.log.Error("can't find user for course import contact", "email", contact.GetEmail(), "err", err)
			continue
		}
		// Wait a bit, otherwise LDAP locks us out -- carried over from the v1 handler.
		time.Sleep(time.Millisecond * 200)
		user.Name = contact.GetFirstName() + " " + contact.GetLastName()
		user.Role = model.LecturerType
		if err := a.dao.UsersDao.UpsertUser(user); err != nil {
			a.log.Error("can't upsert user for course import contact", "err", err)
			continue
		}
		admins = append(admins, user)
	}

	for _, admin := range admins {
		if err := a.dao.CoursesDao.AddAdminToCourse(admin.ID, course.ID); err != nil {
			a.log.Error("can't add admin to imported course", "err", err)
		}
		subject := fmt.Sprintf("Vorlesungsstreams %s | Lecture streaming %s", course.Name, course.Name)
		if err := a.notifyCourseImported(ctx, courseImportMailData{
			Name: admin.Name, Course: course, Users: admins, OptIn: optIn,
		}, admin.Email.String, subject); err != nil {
			a.log.Error("can't send course import email", "course", course.Name, "email", admin.Email.String, "err", err)
		}
		// A tenth of a second delay, being nice to our mailrelay -- carried over from
		// the v1 handler.
		time.Sleep(time.Millisecond * 100)
	}

	return nil
}

// courseImportMailData feeds apiv2/server/template/mail-course-registered.gotemplate,
// the invitation sent to a newly-imported course's main contact.
type courseImportMailData struct {
	Name   string
	OptIn  bool
	Course model.Course
	Users  []*model.User
}

func (a *API) notifyCourseImported(ctx context.Context, d courseImportMailData, mailAddr string, subject string) error {
	tmpl, err := template.ParseFS(courseImportTemplateFS, "template/*.gotemplate")
	if err != nil {
		return err
	}
	var body bytes.Buffer
	if err := tmpl.ExecuteTemplate(&body, "mail-course-registered.gotemplate", d); err != nil {
		return err
	}
	return a.dao.EmailDao.Create(ctx, &model.Email{
		From:    tools.Cfg.Mail.Sender,
		To:      mailAddr,
		Subject: subject,
		Body:    body.String(),
	})
}
