package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"sort"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// The course administration page's RPCs, other than those about single lectures.
// Except for ListAdministeredCourses, each is gated by requiresCourseAdmin in
// services.go, which has already checked the caller administers the course.

// courseVisibilities are the values a course's visibility may take. v1 matched them
// with an unanchored regexp, so anything containing one of them was stored.
var courseVisibilities = map[string]bool{
	"public":   true,
	"loggedin": true,
	"enrolled": true,
	"hidden":   true,
}

// maxInvitees bounds one inviteCourseParticipants call; every invitee is a lookup,
// a write or two and a queued email.
const maxInvitees = 200

// administeredCourse loads the course a course-scoped request names. The policy has
// already refused a course the caller does not administer; course 0 is refused here
// again because it is how the statistics queries spell "every course", and no
// handler below should ever act on it.
func (a *API) administeredCourse(ctx context.Context, courseID uint32) (model.Course, error) {
	if courseID == serverStatsCourseID {
		return model.Course{}, e.WithStatus(http.StatusNotFound, errNoSuchCourse)
	}

	course, err := a.dao.CoursesDao.GetCourseById(ctx, uint(courseID))
	if err != nil {
		return model.Course{}, e.FromGorm(err, "can't find course")
	}
	// Find reports a missing row as a zero-value course and a nil error.
	if course.ID == 0 {
		return model.Course{}, e.WithStatus(http.StatusNotFound, errNoSuchCourse)
	}
	return course, nil
}

// refuseCourseZero is administeredCourse's check for handlers that do not need the
// course itself.
func refuseCourseZero(courseID uint32) error {
	if courseID == serverStatsCourseID {
		return e.WithStatus(http.StatusNotFound, errNoSuchCourse)
	}
	return nil
}

// GetCourseAdmin replaces what web.AdminPage rendered into the course settings tab.
func (a *API) GetCourseAdmin(ctx context.Context, req *protobuf.GetCourseAdminRequest) (*protobuf.CourseAdmin, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}
	return courseAdminMessage(course), nil
}

// UpdateCourseSettings replaces web.UpdateCourse, the settings tab's form post. Only
// the fields present change; v1's form sent every one of them, unticked boxes as off.
//
// v1's form also carried the "reload from TUMOnline" buttons, but their labels no
// longer matched what the handler compared against, so they saved the settings
// instead. They are left out here rather than ported broken.
func (a *API) UpdateCourseSettings(ctx context.Context, req *protobuf.UpdateCourseSettingsRequest) (*protobuf.CourseAdmin, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	if req.Visibility != nil && !courseVisibilities[req.GetVisibility()] {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("visibility must be public, loggedin, enrolled or hidden"))
	}

	var columns, changes []string
	set := func(column string, value any) {
		columns = append(columns, column)
		changes = append(changes, fmt.Sprintf("%s=%v", column, value))
	}
	if req.Visibility != nil {
		course.Visibility = req.GetVisibility()
		set("visibility", course.Visibility)
	}
	if req.VodEnabled != nil {
		course.VODEnabled = req.GetVodEnabled()
		set("vod_enabled", course.VODEnabled)
	}
	if req.DownloadsEnabled != nil {
		course.DownloadsEnabled = req.GetDownloadsEnabled()
		set("downloads_enabled", course.DownloadsEnabled)
	}
	if req.ChatEnabled != nil {
		course.ChatEnabled = req.GetChatEnabled()
		set("chat_enabled", course.ChatEnabled)
	}
	if req.AnonymousChatEnabled != nil {
		course.AnonymousChatEnabled = req.GetAnonymousChatEnabled()
		set("anonymous_chat_enabled", course.AnonymousChatEnabled)
	}
	if req.ModeratedChatEnabled != nil {
		course.ModeratedChatEnabled = req.GetModeratedChatEnabled()
		set("moderated_chat_enabled", course.ModeratedChatEnabled)
	}
	if req.LivePrivate != nil {
		course.LivePrivate = req.GetLivePrivate()
		set("live_private", course.LivePrivate)
	}
	if req.VodPrivate != nil {
		course.VodPrivate = req.GetVodPrivate()
		set("vod_private", course.VodPrivate)
	}

	if len(columns) == 0 {
		return courseAdminMessage(course), nil
	}

	if err := a.dao.CoursesDao.UpdateCourseColumns(ctx, course, columns...); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// v1 left no trace of a settings change; visibility is worth one.
	a.audit(ctx, model.AuditCourseEdit, fmt.Sprintf("%s:'%s' settings: %s", course.Name, course.Slug, strings.Join(changes, ", ")))

	return courseAdminMessage(course), nil
}

// CopyCourse replaces v1's copyCourse: the course's settings, every lecture and the
// administrators, into a new course in another semester.
func (a *API) CopyCourse(ctx context.Context, req *protobuf.CopyCourseRequest) (*protobuf.CopyCourseResponse, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	if req.GetYear() < 1000 || req.GetYear() > 9999 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the year must have four digits"))
	}
	term := req.GetTerm()
	if term != "W" && term != "S" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the term must be W or S"))
	}

	// v1 did not check, and created a second course under the same URL.
	_, err = a.dao.CoursesDao.GetCourseBySlugYearAndTerm(ctx, course.Slug, term, int(req.GetYear()))
	switch {
	case err == nil:
		return nil, e.WithStatus(http.StatusConflict, fmt.Errorf("this semester already has a course with the slug %q", course.Slug))
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	admins, err := a.dao.CoursesDao.GetCourseAdmins(course.ID)
	if err != nil {
		// As v1: the copy is still worth having, and its admins can be added after.
		a.log.Error("can't get course admins to copy", "err", err)
		admins = nil
	}

	streams := course.Streams
	copied := course
	copied.Model = gorm.Model{}
	copied.Streams = nil
	copied.Year = int(req.GetYear())
	copied.TeachingTerm = term
	if err := a.dao.CoursesDao.CreateCourse(ctx, &copied, true); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	numErrors := int32(0)
	for _, stream := range streams {
		stream.Model = gorm.Model{}
		stream.CourseID = copied.ID
		// Loaded with the course, these still carry the original's primary keys, so
		// creating them would only collide; v1's copies ended up without them too.
		stream.Files = nil
		stream.VideoSections = nil
		stream.TranscodingProgresses = nil
		if err := a.dao.StreamsDao.CreateStream(&stream); err != nil {
			a.log.Error("can't copy stream", "err", err)
			numErrors++
		}
	}
	for _, admin := range admins {
		if err := a.dao.CoursesDao.AddAdminToCourse(admin.ID, copied.ID); err != nil {
			a.log.Error("can't copy course admin", "err", err)
			numErrors++
		}
	}

	a.audit(ctx, model.AuditCourseCreate, fmt.Sprintf("%s:'%s' (%d, %s) copied from [%d]", copied.Slug, copied.Name, copied.Year, copied.TeachingTerm, course.ID))

	return &protobuf.CopyCourseResponse{CourseId: uint32(copied.ID), NumErrors: numErrors}, nil
}

// DeleteCourse replaces v1's deleteCourse.
func (a *API) DeleteCourse(ctx context.Context, req *protobuf.DeleteCourseRequest) (*emptypb.Empty, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	a.log.Info("deleting course", "course", course.ID)
	a.audit(ctx, model.AuditCourseDelete, fmt.Sprintf("'%s' (%d, %s)[%d]", course.Name, course.Year, course.TeachingTerm, course.ID))

	// Reports nothing; it logs what it could not delete and carries on, as in v1.
	a.dao.CoursesDao.DeleteCourse(course)
	dao.Cache.Clear()

	return &emptypb.Empty{}, nil
}

// ListCourseAdmins replaces v1's getAdmins.
func (a *API) ListCourseAdmins(ctx context.Context, req *protobuf.ListCourseAdminsRequest) (*protobuf.ListCourseAdminsResponse, error) {
	if err := refuseCourseZero(req.GetCourseId()); err != nil {
		return nil, err
	}

	admins, err := a.dao.CoursesDao.GetCourseAdmins(uint(req.GetCourseId()))
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.CourseAdminUser, 0, len(admins))
	for _, admin := range admins {
		out = append(out, courseAdminUserMessage(admin))
	}
	return &protobuf.ListCourseAdminsResponse{Admins: out}, nil
}

// AddCourseAdmin replaces v1's addAdminToCourse, including promoting a student or
// external participant to lecturer: without that role the administration pages
// that require the lecture permission would refuse them.
func (a *API) AddCourseAdmin(ctx context.Context, req *protobuf.AddCourseAdminRequest) (*protobuf.CourseAdminUser, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	user, err := a.dao.UsersDao.GetUserByID(ctx, uint(req.GetUserId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find user")
	}
	// GetUserByID's Find reports a missing row as a zero-value user.
	if user.ID == 0 {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such user"))
	}

	if err := a.dao.CoursesDao.AddAdminToCourse(user.ID, course.ID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if user.Role == model.GenericType || user.Role == model.StudentType {
		user.Role = model.LecturerType
		if err := a.dao.UsersDao.UpdateUser(user); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	}
	// Cached for ten seconds; the new admin's next request should see the course.
	dao.InvalidateUserCache(user.ID)

	a.audit(ctx, model.AuditCourseEdit, fmt.Sprintf("%s:'%s' add: %s (%d)", course.Name, course.Slug, user.GetPreferredName(), user.ID))

	return courseAdminUserMessage(user), nil
}

// RemoveCourseAdmin replaces v1's removeAdminFromCourse, which refused to remove the
// last administrator: nobody but a server administrator could then reach the course.
func (a *API) RemoveCourseAdmin(ctx context.Context, req *protobuf.RemoveCourseAdminRequest) (*protobuf.CourseAdminUser, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	admins, err := a.dao.CoursesDao.GetCourseAdmins(course.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if len(admins) == 1 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("can not remove the course's last administrator"))
	}

	var user *model.User
	for i := range admins {
		if admins[i].ID == uint(req.GetUserId()) {
			user = &admins[i]
			break
		}
	}
	if user == nil {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("not an administrator of this course"))
	}

	if err := a.dao.CoursesDao.RemoveAdminFromCourse(user.ID, course.ID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	dao.InvalidateUserCache(user.ID)

	a.audit(ctx, model.AuditCourseEdit, fmt.Sprintf("%s:'%s' remove: %s (%d)", course.Name, course.Slug, user.GetPreferredName(), user.ID))

	return courseAdminUserMessage(*user), nil
}

// SearchUsersForCourse replaces v1's searchUserForCourse for finding someone to make
// an administrator. searchUsers needs users.manage, which a course's lecturers lack;
// this answers no more than v1 did: name, login and role.
func (a *API) SearchUsersForCourse(ctx context.Context, req *protobuf.SearchUsersForCourseRequest) (*protobuf.SearchUsersForCourseResponse, error) {
	if err := refuseCourseZero(req.GetCourseId()); err != nil {
		return nil, err
	}

	query := strings.TrimSpace(searchQueryAllowed.ReplaceAllString(req.GetQ(), ""))
	if len(query) < searchQueryMinLength {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("query too short (minimum length is 3)"))
	}

	users, err := a.dao.UsersDao.SearchUser(query)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.CourseAdminUser, 0, len(users))
	for _, user := range users {
		out = append(out, courseAdminUserMessage(user))
	}
	return &protobuf.SearchUsersForCourseResponse{Users: out}, nil
}

// ListCourseLectureHallSettings replaces v1's lecture-halls-by-id.
func (a *API) ListCourseLectureHallSettings(ctx context.Context, req *protobuf.ListCourseLectureHallSettingsRequest) (*protobuf.ListCourseLectureHallSettingsResponse, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	return &protobuf.ListCourseLectureHallSettingsResponse{
		LectureHalls: courseLectureHallSettings(course, a.courseLectureHalls(course)),
	}, nil
}

// UpdateCourseLectureHallSettings replaces v1's presets POST. As there, what is sent
// replaces every hall's settings. Unlike there, a hall must be one the course has
// lectures in and a preset one of that hall's: v1 stored whatever it was sent.
func (a *API) UpdateCourseLectureHallSettings(ctx context.Context, req *protobuf.UpdateCourseLectureHallSettingsRequest) (*protobuf.ListCourseLectureHallSettingsResponse, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	halls := a.courseLectureHalls(course)
	byID := make(map[uint]model.LectureHall, len(halls))
	for _, hall := range halls {
		byID[hall.ID] = hall
	}

	seen := map[uint]bool{}
	sources := make([]model.SourcePreference, 0, len(req.GetLectureHalls()))
	presets := make([]model.CameraPresetPreference, 0, len(req.GetLectureHalls()))
	for _, update := range req.GetLectureHalls() {
		hallID := uint(update.GetLectureHallId())
		hall, ok := byID[hallID]
		if !ok {
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("the course has no lectures in lecture hall %d", hallID))
		}
		if seen[hallID] {
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("lecture hall %d is listed twice", hallID))
		}
		seen[hallID] = true

		mode := model.SourceMode(update.GetSourceMode())
		if mode < model.SourceModeCOMB || mode > model.SourceModeCAMOnly {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("unknown source mode"))
		}
		sources = append(sources, model.SourcePreference{LectureHallID: hallID, SourceMode: mode})

		// As v1: 0 means no preference, and the hall's default preset applies.
		presetID := int(update.GetSelectedPresetId())
		if presetID == 0 {
			continue
		}
		if !hasPreset(hall, presetID) {
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("lecture hall %d has no preset %d", hallID, presetID))
		}
		presets = append(presets, model.CameraPresetPreference{LectureHallID: hallID, PresetID: presetID})
	}

	course.SetCameraPresetPreference(presets)
	course.SetSourcePreference(sources)
	if err := a.dao.CoursesDao.UpdateCourseColumns(ctx, course, "camera_preset_preferences", "source_preferences"); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	a.audit(ctx, model.AuditCourseEdit, fmt.Sprintf("%s:'%s'", course.Name, course.Slug))

	return &protobuf.ListCourseLectureHallSettingsResponse{
		LectureHalls: courseLectureHallSettings(course, halls),
	}, nil
}

// courseLectureHalls loads the halls the course has lectures in, by name. A hall that
// cannot be loaded is left out, as v1 did.
func (a *API) courseLectureHalls(course model.Course) []model.LectureHall {
	ids := map[uint]bool{}
	for _, stream := range course.Streams {
		if stream.LectureHallID != 0 {
			ids[stream.LectureHallID] = true
		}
	}

	halls := make([]model.LectureHall, 0, len(ids))
	for id := range ids {
		hall, err := a.dao.LectureHallsDao.GetLectureHallByID(id)
		if err != nil {
			a.log.Error("can't fetch lecture hall of a course's stream", "err", err, "lectureHall", id)
			continue
		}
		halls = append(halls, hall)
	}
	// v1 answered in map order; the page lists them, so it should not shuffle.
	sort.Slice(halls, func(i, j int) bool { return halls[i].Name < halls[j].Name })
	return halls
}

func hasPreset(hall model.LectureHall, presetID int) bool {
	for _, preset := range hall.CameraPresets {
		if preset.PresetID == presetID {
			return true
		}
	}
	return false
}

func courseLectureHallSettings(course model.Course, halls []model.LectureHall) []*protobuf.CourseLectureHallSetting {
	preferred := map[uint]int{}
	for _, pref := range course.GetCameraPresetPreference() {
		if _, ok := preferred[pref.LectureHallID]; !ok {
			preferred[pref.LectureHallID] = pref.PresetID
		}
	}

	out := make([]*protobuf.CourseLectureHallSetting, 0, len(halls))
	for _, hall := range halls {
		presets := make([]*protobuf.CourseLectureHallPreset, 0, len(hall.CameraPresets))
		for _, preset := range hall.CameraPresets {
			presets = append(presets, &protobuf.CourseLectureHallPreset{
				PresetId:  int32(preset.PresetID),
				Name:      preset.Name,
				Image:     preset.Image,
				IsDefault: preset.IsDefault,
			})
		}

		setting := &protobuf.CourseLectureHallSetting{
			LectureHallId:   uint32(hall.ID),
			LectureHallName: hall.Name,
			Presets:         presets,
			SourceMode:      protobuf.CourseSourceMode(course.GetSourceModeForLectureHall(hall.ID)),
		}
		// As v1: a preference only counts for a hall that has presets.
		if len(hall.CameraPresets) != 0 {
			setting.SelectedPresetId = int32(preferred[hall.ID])
		}
		out = append(out, setting)
	}
	return out
}

// ListCourseParticipants replaces the invitations table v1 rendered into the page:
// the external accounts enrolled in the course.
func (a *API) ListCourseParticipants(ctx context.Context, req *protobuf.ListCourseParticipantsRequest) (*protobuf.ListCourseParticipantsResponse, error) {
	if err := refuseCourseZero(req.GetCourseId()); err != nil {
		return nil, err
	}

	course := model.Course{Model: gorm.Model{ID: uint(req.GetCourseId())}}
	if err := a.dao.CoursesDao.GetInvitedUsersForCourse(&course); err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}

	out := make([]*protobuf.CourseParticipant, 0, len(course.Users))
	for _, user := range course.Users {
		// The template skipped these too; an invitation always has an address.
		if !user.Email.Valid || user.Email.String == "" {
			continue
		}
		out = append(out, &protobuf.CourseParticipant{
			Id:           uint32(user.ID),
			Name:         user.GetPreferredName(),
			Email:        user.Email.String,
			AccountSetUp: user.Password != "",
		})
	}
	return &protobuf.ListCourseParticipantsResponse{Participants: out}, nil
}

// InviteCourseParticipants replaces v1's createUserForCourse, both its single and
// its batch form. An address without an account gets an external account, enrolled,
// and the invitation to set a password; one with an account is enrolled and told.
//
// v1 ran a batch in the background, two seconds apart, to pace the mail. The mails
// are queued and sent at a configured rate by the mail cron now, so this answers once
// every invitee is handled, with how each went.
func (a *API) InviteCourseParticipants(ctx context.Context, req *protobuf.InviteCourseParticipantsRequest) (*protobuf.InviteCourseParticipantsResponse, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	invitees := req.GetInvitees()
	if len(invitees) == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("nobody to invite"))
	}
	if len(invitees) > maxInvitees {
		return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("at most %d invitees per call", maxInvitees))
	}

	// All or nothing on the input, so a typo in line 40 does not leave 39 invited.
	type invitee struct{ name, email string }
	valid := make([]invitee, 0, len(invitees))
	for i, inv := range invitees {
		name := strings.TrimSpace(inv.GetName())
		email := strings.TrimSpace(inv.GetEmail())
		if name == "" {
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("invitee %d has no name", i+1))
		}
		if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("invitee %d has no valid email address", i+1))
		}
		valid = append(valid, invitee{name: name, email: email})
	}

	results := make([]*protobuf.CourseInvitationResult, 0, len(valid))
	for _, inv := range valid {
		created, err := a.inviteParticipant(ctx, course, inv.name, inv.email)
		result := &protobuf.CourseInvitationResult{Email: inv.email, AccountCreated: created}
		if err != nil {
			a.log.Error("can't invite course participant", "err", err, "course", course.ID)
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return &protobuf.InviteCourseParticipantsResponse{Results: results}, nil
}

// inviteParticipant is v1's addSingleUserToCourse, reporting whether it created the
// account.
func (a *API) inviteParticipant(ctx context.Context, course model.Course, name, email string) (bool, error) {
	existing, err := a.dao.UsersDao.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		if err := a.dao.CoursesDao.AddUserToCourse(ctx, existing.ID, course.ID); err != nil {
			return false, errors.New("could not enrol the existing account")
		}
		dao.InvalidateUserCache(existing.ID)

		// The same mail v1 sent an account that already existed.
		if err := a.dao.EmailDao.Create(ctx, &model.Email{
			From:    tools.Cfg.Mail.Sender,
			To:      email,
			Subject: "Setup your TUM-Live account",
			Body: fmt.Sprintf("Hello!\n"+
				"You have been invited to participate in the course \"%s\" on TUM-Live. Check it out at https://live.rbg.tum.de/",
				course.Name),
		}); err != nil {
			return false, errors.New("enrolled, but the notification could not be queued")
		}
		return false, nil

	case !errors.Is(err, gorm.ErrRecordNotFound):
		// v1 read any failed lookup as "no account" and created a second one.
		return false, errors.New("could not look the address up")
	}

	user := model.User{
		Name:  name,
		Email: sql.NullString{String: email, Valid: true},
		Role:  model.GenericType,
	}
	if err := a.dao.UsersDao.CreateUser(ctx, &user); err != nil {
		return false, errors.New("could not create the account")
	}
	if err := a.dao.CoursesDao.AddUserToCourse(ctx, user.ID, course.ID); err != nil {
		return true, errors.New("account created, but it could not be enrolled")
	}
	if err := tools.SendAccountInvite(ctx, a.dao, email); err != nil {
		a.log.Error("participant created but the invitation could not be sent", "err", err)
		return true, errors.New("account created, but the invitation could not be sent")
	}
	return true, nil
}

// ListAdministeredCourses replaces the course list web.AdminPage rendered into the
// administration sidebar, every semester's. Gated on the lecture permission by its
// policy.
func (a *API) ListAdministeredCourses(ctx context.Context, _ *emptypb.Empty) (*protobuf.ListAdministeredCoursesResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	// "" and 0 are every semester.
	courses, err := a.dao.CoursesDao.GetAdministeredCoursesByUserId(ctx, user.ID, "", 0)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// Newest semester first; a year's winter term starts after its summer term.
	sort.SliceStable(courses, func(i, j int) bool {
		if courses[i].Year != courses[j].Year {
			return courses[i].Year > courses[j].Year
		}
		if courses[i].TeachingTerm != courses[j].TeachingTerm {
			return courses[i].TeachingTerm == "W"
		}
		return courses[i].Name < courses[j].Name
	})

	out := make([]*protobuf.CourseAdminSummary, 0, len(courses))
	for _, course := range courses {
		out = append(out, &protobuf.CourseAdminSummary{
			Id:   uint32(course.ID),
			Name: course.Name,
			Slug: course.Slug,
			Year: int32(course.Year),
			Term: course.TeachingTerm,
		})
	}
	return &protobuf.ListAdministeredCoursesResponse{Courses: out}, nil
}

func courseAdminMessage(c model.Course) *protobuf.CourseAdmin {
	return &protobuf.CourseAdmin{
		Id:                   uint32(c.ID),
		Name:                 c.Name,
		Slug:                 c.Slug,
		Year:                 int32(c.Year),
		Term:                 c.TeachingTerm,
		Visibility:           c.Visibility,
		VodEnabled:           c.VODEnabled,
		DownloadsEnabled:     c.DownloadsEnabled,
		ChatEnabled:          c.ChatEnabled,
		AnonymousChatEnabled: c.AnonymousChatEnabled,
		ModeratedChatEnabled: c.ModeratedChatEnabled,
		VodChatEnabled:       c.VodChatEnabled,
		LivePrivate:          c.LivePrivate,
		VodPrivate:           c.VodPrivate,
		Language:             c.Language.String,
		TumOnlineIdentifier:  c.TUMOnlineIdentifier,
		UserId:               uint32(c.UserID),
	}
}

func courseAdminUserMessage(u model.User) *protobuf.CourseAdminUser {
	return &protobuf.CourseAdminUser{
		Id:    uint32(u.ID),
		Name:  u.GetPreferredName(),
		Login: u.GetLoginString(),
		Role:  uint32(u.Role),
	}
}
