package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// checkSameOrigin validates that the request originates from the same host.
// It reads the Origin header (falling back to Referer) and compares the
// scheme+host to c.Request.Host. Returns true when the origin matches.
// Requests with no Origin AND no Referer header are rejected (safe default).
func checkSameOrigin(c *gin.Context) bool {
	origin := c.Request.Header.Get("Origin")
	if origin == "" {
		// Fall back to Referer, using only the scheme+host portion.
		ref := c.Request.Header.Get("Referer")
		if ref == "" {
			return false
		}
		// Strip everything from the third slash onward to get scheme+host.
		// e.g. "https://foo.example.com/path" → "https://foo.example.com"
		if after, ok := strings.CutPrefix(ref, "https://"); ok {
			if slash := strings.Index(after, "/"); slash >= 0 {
				origin = "https://" + after[:slash]
			} else {
				origin = "https://" + after
			}
		} else if after, ok := strings.CutPrefix(ref, "http://"); ok {
			if slash := strings.Index(after, "/"); slash >= 0 {
				origin = "http://" + after[:slash]
			} else {
				origin = "http://" + after
			}
		} else {
			return false
		}
	}
	// Compare origin scheme+host to the request's own Host header.
	// c.Request.Host is already scheme-stripped; we must compare without scheme.
	reqHost := c.Request.Host
	var originHost string
	if after, ok := strings.CutPrefix(origin, "https://"); ok {
		originHost = after
	} else if after, ok := strings.CutPrefix(origin, "http://"); ok {
		originHost = after
	} else {
		return false
	}
	return strings.EqualFold(originHost, reqHost)
}

// IntegrationConfirmPageData holds the data rendered by the confirmation template.
type IntegrationConfirmPageData struct {
	IndexData   IndexData
	CourseName  string
	CourseID    uint
	ServiceUser model.User
	Redirect    string
}

// checkServiceAccountAllowed validates the "service" parameter against the
// configured IntegrationServiceAccountID. It returns the parsed serviceID on
// success, or writes an error response and returns false when the request must
// be rejected.
//
// Fail-closed policy: if IntegrationServiceAccountID is 0 (unset), the
// binding-approval page is disabled entirely (no service account configured).
// If it is non-zero, only the exact matching service param is permitted.
func checkServiceAccountAllowed(c *gin.Context, serviceIDStr string) (uint, bool) {
	serviceID, err := strconv.ParseUint(serviceIDStr, 10, 64)
	if err != nil || serviceIDStr == "" {
		c.Status(http.StatusBadRequest)
		tools.RenderErrorPage(c, http.StatusBadRequest, "Missing or invalid 'service' parameter.")
		return 0, false
	}
	allowed := tools.Cfg.IntegrationServiceAccountID
	if allowed == 0 {
		c.Status(http.StatusForbidden)
		tools.RenderErrorPage(c, http.StatusForbidden, "Integration binding is not available: integrationServiceAccountID must be configured.")
		return 0, false
	}
	if uint(serviceID) != allowed {
		c.Status(http.StatusForbidden)
		tools.RenderErrorPage(c, http.StatusForbidden, "This service account is not the configured integration account.")
		return 0, false
	}
	return uint(serviceID), true
}

// IntegrationBindingConfirmGET renders the approval page that shows the course
// name and service-account name, and asks the course admin to confirm the binding.
//
// Query params:
//   - service: numeric user ID of the ServiceType account to bind
//   - redirect: URL to return to after confirmation (validated against config)
func (r mainRoutes) IntegrationBindingConfirmGET(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	serviceID, ok := checkServiceAccountAllowed(c, c.Query("service"))
	if !ok {
		return
	}

	serviceUser, err := r.UsersDao.GetUserByID(context.Background(), serviceID)
	if err != nil {
		// Distinguish a genuine not-found from a backend/DB error. GetUserByID
		// uses Find, so a missing row normally returns (zero user, nil err)
		// (handled by the ID == 0 check below); but if the DAO ever surfaces
		// gorm.ErrRecordNotFound, treat it as 404, and any other error as 500.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			tools.RenderErrorPage(c, http.StatusNotFound, "Service account not found.")
			return
		}
		c.Status(http.StatusInternalServerError)
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Failed to look up service account.")
		return
	}
	if serviceUser.ID == 0 {
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
//
// Same-origin enforcement: the handler requires an Origin (or Referer) header
// whose host matches c.Request.Host. Mismatches are rejected with 403.
func (r mainRoutes) IntegrationBindingConfirmPOST(c *gin.Context) {
	if !checkSameOrigin(c) {
		c.Status(http.StatusForbidden)
		tools.RenderErrorPage(c, http.StatusForbidden, "Cross-origin requests are not permitted.")
		return
	}

	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	serviceID, ok := checkServiceAccountAllowed(c, c.PostForm("service"))
	if !ok {
		return
	}

	serviceUser, err := r.UsersDao.GetUserByID(context.Background(), serviceID)
	if err != nil {
		// Distinguish a genuine not-found from a backend/DB error. GetUserByID
		// uses Find, so a missing row normally returns (zero user, nil err)
		// (handled by the ID == 0 check below); but if the DAO ever surfaces
		// gorm.ErrRecordNotFound, treat it as 404, and any other error as 500.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			tools.RenderErrorPage(c, http.StatusNotFound, "Service account not found.")
			return
		}
		c.Status(http.StatusInternalServerError)
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Failed to look up service account.")
		return
	}
	if serviceUser.ID == 0 {
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

	if err := r.CoursesDao.AddAdminToCourse(serviceUser.ID, course.ID); err != nil {
		logger.Error("could not add service account as course admin", "err", err)
		c.Status(http.StatusInternalServerError)
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Could not complete the binding. Please try again.")
		return
	}

	// Write audit log only after a successful grant, matching the pattern in
	// api/courses.go:addAdminToCourse, so a failed AddAdminToCourse never
	// records a false "binding succeeded" entry.
	if err := r.AuditDao.Create(&model.Audit{
		User:    tumLiveContext.User,
		Message: fmt.Sprintf("integration binding: %s:'%s' → service account %s (%d)", course.Name, course.Slug, serviceUser.GetPreferredName(), serviceUser.ID),
		Type:    model.AuditCourseEdit,
	}); err != nil {
		logger.Error("Create audit (integration binding)", "err", err)
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
