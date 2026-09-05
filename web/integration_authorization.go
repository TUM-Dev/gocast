package web

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

const integrationCodeLifetime = 5 * time.Minute

type integrationAuthorizationData struct {
	IndexData   IndexData
	Integration model.Integration
	Courses     []model.Course
	State       string
}

func validAuthorizationState(state string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	return raw, err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == state
}

func integrationPathID(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 32)
	return uint(id), err == nil && id != 0
}

func canAuthorizeCourse(user *model.User, course model.Course) bool {
	if user == nil || user.ID == 0 {
		return false
	}
	if course.UserID == user.ID {
		return true
	}
	for _, admin := range course.Admins {
		if admin.ID == user.ID {
			return true
		}
	}
	return false
}

func (r mainRoutes) integrationAuthorizationPage(c *gin.Context) {
	integrationID, ok := integrationPathID(c, "id")
	state := c.Query("state")
	if _, valid := validAuthorizationState(state); !ok || !valid {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "This authorization link is invalid. Return to the application and start again.")
		return
	}
	integration, err := r.IntegrationGrantDao.GetIntegrationByID(c, integrationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			tools.RenderErrorPage(c, http.StatusNotFound, "This authorization request is no longer available. Return to the application and start again.")
		} else {
			logger.Error("could not load integration authorization", "err", err)
			c.Status(http.StatusInternalServerError)
			tools.RenderErrorPage(c, http.StatusInternalServerError, "We couldn't load this authorization request. Reload this page to try again.")
		}
		return
	}
	user := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	courses, err := r.IntegrationGrantDao.GetAuthorizableCourses(c, user.ID)
	if err != nil {
		logger.Error("could not load authorizable courses", "err", err)
		c.Status(http.StatusInternalServerError)
		tools.RenderErrorPage(c, http.StatusInternalServerError, "We couldn't load your courses. Reload this page to try again.")
		return
	}
	data := integrationAuthorizationData{
		IndexData: NewIndexDataWithContext(c), Integration: integration,
		Courses: courses, State: state,
	}
	if err := templateExecutor.ExecuteTemplate(c.Writer, "integration-authorization.gohtml", data); err != nil {
		logger.Error("could not render integration authorization", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func (r mainRoutes) authorizeIntegration(c *gin.Context) {
	integrationID, ok := integrationPathID(c, "id")
	state := c.PostForm("state")
	stateRaw, validState := validAuthorizationState(state)
	if !ok || !validState {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "This authorization form is invalid. Return to the application and start again.")
		return
	}
	integration, err := r.IntegrationGrantDao.GetIntegrationByID(c, integrationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			tools.RenderErrorPage(c, http.StatusNotFound, "This authorization request is no longer available. Return to the application and start again.")
		} else {
			logger.Error("could not load integration authorization", "err", err)
			c.Status(http.StatusInternalServerError)
			tools.RenderErrorPage(c, http.StatusInternalServerError, "We couldn't load this authorization request. Go back and reload the authorization page.")
		}
		return
	}
	if c.PostForm("decision") == "cancel" {
		c.Redirect(http.StatusFound, integrationReturnURL(integration.ReturnURL, state, "error", "access_denied"))
		return
	}
	if c.PostForm("decision") != "approve" {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "Go back to the authorization page, choose whether to authorize or cancel, and try again.")
		return
	}
	courseID, ok := integrationPathFormID(c, "course_id")
	if !ok {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "Go back to the authorization page, choose a course, and try again.")
		return
	}
	user := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	course, err := r.IntegrationGrantDao.GetCourseForAuthorization(c, courseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			tools.RenderErrorPage(c, http.StatusNotFound, "That course is no longer available. Go back and reload the authorization page.")
		} else {
			logger.Error("could not load course for integration authorization", "err", err)
			c.Status(http.StatusInternalServerError)
			tools.RenderErrorPage(c, http.StatusInternalServerError, "We couldn't load that course. Go back and reload the authorization page.")
		}
		return
	}
	if !canAuthorizeCourse(user, course) {
		c.Status(http.StatusForbidden)
		tools.RenderErrorPage(c, http.StatusForbidden, "You no longer have permission to authorize applications for this course. Go back and reload the authorization page.")
		return
	}
	codeRaw := make([]byte, 32)
	if _, err := rand.Read(codeRaw); err != nil {
		logger.Error("could not create integration authorization code", "err", err)
		c.Status(http.StatusInternalServerError)
		tools.RenderErrorPage(c, http.StatusInternalServerError, "We couldn't create this authorization. Go back to the authorization page and try again.")
		return
	}
	codeHash, stateHash := sha256.Sum256(codeRaw), sha256.Sum256(stateRaw)
	if _, err := r.IntegrationGrantDao.ApproveIntegrationCourse(c, integrationID, courseID, codeHash[:], stateHash[:], time.Now().Add(integrationCodeLifetime)); err != nil {
		logger.Error("could not save integration authorization", "err", err)
		c.Status(http.StatusInternalServerError)
		tools.RenderErrorPage(c, http.StatusInternalServerError, "We couldn't save this authorization. Go back to the authorization page and try again.")
		return
	}
	code := base64.RawURLEncoding.EncodeToString(codeRaw)
	c.Redirect(http.StatusFound, integrationReturnURL(integration.ReturnURL, state, "code", code))
}

func integrationPathFormID(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.PostForm(name), 10, 32)
	return uint(id), err == nil && id != 0
}

func integrationReturnURL(raw, state, resultName, result string) string {
	u, _ := url.Parse(raw)
	query := u.Query()
	query.Set("state", state)
	query.Set(resultName, result)
	u.RawQuery = query.Encode()
	return u.String()
}
