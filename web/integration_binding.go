package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// IntegrationConfirmPageData holds the data rendered by the confirmation template.
type IntegrationConfirmPageData struct {
	IndexData   IndexData
	CourseName  string
	CourseID    uint
	ServiceUser model.User
	Redirect    string
}

// IntegrationBindingConfirmGET renders the approval page that shows the course
// name and service-account name, and asks the course admin to confirm the binding.
//
// Query params:
//   - service: numeric user ID of the ServiceType account to bind
//   - redirect: URL to return to after confirmation (validated against config)
func (r mainRoutes) IntegrationBindingConfirmGET(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	serviceIDStr := c.Query("service")
	serviceID, err := strconv.ParseUint(serviceIDStr, 10, 64)
	if err != nil || serviceIDStr == "" {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "Missing or invalid 'service' parameter.")
		return
	}

	serviceUser, err := r.UsersDao.GetUserByID(context.Background(), uint(serviceID))
	if err != nil {
		c.Status(http.StatusNotFound)
		tools.RenderErrorPage(c, http.StatusNotFound, "Service account not found.")
		return
	}
	if serviceUser.Role != model.ServiceType {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "The specified user is not a service account.")
		return
	}

	redirect := c.Query("redirect")

	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext

	if err := templateExecutor.ExecuteTemplate(c.Writer, "integration-confirm.gohtml", IntegrationConfirmPageData{
		IndexData:   indexData,
		CourseName:  tumLiveContext.Course.Name,
		CourseID:    tumLiveContext.Course.ID,
		ServiceUser: serviceUser,
		Redirect:    redirect,
	}); err != nil {
		logger.Error("Error executing template integration-confirm.gohtml", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

// IntegrationBindingConfirmPOST processes the course-binding approval form.
//
// Form fields:
//   - service: numeric user ID of the ServiceType account to bind
//   - redirect: URL to return to after confirmation (validated against config)
func (r mainRoutes) IntegrationBindingConfirmPOST(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	serviceIDStr := c.PostForm("service")
	serviceID, err := strconv.ParseUint(serviceIDStr, 10, 64)
	if err != nil || serviceIDStr == "" {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "Missing or invalid 'service' parameter.")
		return
	}

	serviceUser, err := r.UsersDao.GetUserByID(context.Background(), uint(serviceID))
	if err != nil {
		c.Status(http.StatusNotFound)
		tools.RenderErrorPage(c, http.StatusNotFound, "Service account not found.")
		return
	}
	if serviceUser.Role != model.ServiceType {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "The specified user is not a service account.")
		return
	}

	course := tumLiveContext.Course

	// Write audit log, matching the pattern in api/courses.go:addAdminToCourse.
	if err := r.AuditDao.Create(&model.Audit{
		User:    tumLiveContext.User,
		Message: fmt.Sprintf("integration binding: %s:'%s' → service account %s (%d)", course.Name, course.Slug, serviceUser.GetPreferredName(), serviceUser.ID),
		Type:    model.AuditCourseEdit,
	}); err != nil {
		logger.Error("Create audit (integration binding)", "err", err)
	}

	if err := r.CoursesDao.AddAdminToCourse(serviceUser.ID, course.ID); err != nil {
		logger.Error("could not add service account as course admin", "err", err)
		c.Status(http.StatusInternalServerError)
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Could not complete the binding. Please try again.")
		return
	}

	// Open-redirect prevention: only honour the redirect param when it starts
	// with the configured base URL (and that base URL is non-empty).
	redirect := c.PostForm("redirect")
	if safeTarget := safeRedirectTarget(redirect); safeTarget != "" {
		c.Redirect(http.StatusFound, safeTarget)
	} else {
		// Fall back to the course admin page.
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%d", course.ID))
	}
}

// safeRedirectTarget returns target unchanged when it is safe to redirect to:
//   - AllowedIntegrationRedirectBaseURL must be configured (non-empty).
//   - target must start with that base URL.
//   - The match is anchored at a path boundary: the base URL must be followed
//     by '/', '?', '#', or be exactly equal to the target, so that a base URL
//     of "https://a.example.com" does NOT match "https://a.example.com.evil.com".
//
// Returns "" when any condition is not met.
func safeRedirectTarget(target string) string {
	base := tools.Cfg.AllowedIntegrationRedirectBaseURL
	if base == "" || target == "" {
		return ""
	}
	// Exact match.
	if target == base {
		return target
	}
	// Prefix match anchored at a path/query/fragment boundary.
	// Normalise: strip trailing slash from base so we test the character at
	// len(base) in target, which must be '/', '?', or '#'.
	normalBase := strings.TrimRight(base, "/")
	if !strings.HasPrefix(target, normalBase) {
		return ""
	}
	rest := target[len(normalBase):]
	if len(rest) == 0 || rest[0] == '/' || rest[0] == '?' || rest[0] == '#' {
		return target
	}
	return ""
}
