package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// configGinArtemisSSORouter configures the SSO-based authentication router for Artemis
func configGinArtemisSSORouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := artemisSSORoutes{daoWrapper}

	artemisSSO := router.Group("/api/artemis/auth")
	{
		// SSO-based authentication endpoint
		// Artemis authenticates user via TUM SAML, then calls this with user info
		artemisSSO.POST("/sso", routes.authenticateFromSSO)
	}
}

type artemisSSORoutes struct {
	dao.DaoWrapper
}

type artemisSSORequest struct {
	// User information from SAML assertion
	LrzID     string `json:"lrzId" binding:"required"`     // TUM LRZ ID (e.g., "ge12abc")
	MatrNr    string `json:"matrNr"`                       // Matriculation number (optional for staff)
	FirstName string `json:"firstName" binding:"required"` // Given name from SAML
	LastName  string `json:"lastName"`                     // Surname from SAML (optional)
	Email     string `json:"email"`                        // Email (optional, will be constructed if missing)

	// Artemis authentication
	ArtemisAPIKey string `json:"artemisApiKey" binding:"required"` // Shared secret between Artemis and TUM Live
}

type artemisSSOResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Expires string `json:"expires,omitempty"`
	User    struct {
		ID    uint   `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user,omitempty"`
	Courses []courseInfo `json:"courses,omitempty"`
	BaseURL string       `json:"baseUrl,omitempty"`
	Error   string       `json:"error,omitempty"`
}

type courseInfo struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Year        int    `json:"year"`
	Term        string `json:"term"`
	UploadURL   string `json:"uploadUrl"`
	Description string `json:"description"`
}

// authenticateFromSSO authenticates a user based on SAML attributes provided by Artemis
// This is called AFTER Artemis has authenticated the user via TUM SAML SSO
func (r artemisSSORoutes) authenticateFromSSO(c *gin.Context) {
	var req artemisSSORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("Invalid SSO request", "err", err)
		c.JSON(http.StatusBadRequest, artemisSSOResponse{
			Success: false,
			Error:   "missing required fields (lrzId, firstName, artemisApiKey)",
		})
		return
	}

	logger.Info("Artemis SSO authentication attempt", "lrzId", req.LrzID)

	// Step 1: Verify Artemis API key (shared secret)
	expectedAPIKey := tools.Cfg.ArtemisAPIKey
	if expectedAPIKey == "" {
		expectedAPIKey = "e9068cbe-724d-44e6-b707-40f9a57e9046" // Default for development
	}

	if req.ArtemisAPIKey != expectedAPIKey {
		logger.Warn("Invalid Artemis API key", "lrzId", req.LrzID)
		c.JSON(http.StatusUnauthorized, artemisSSOResponse{
			Success: false,
			Error:   "invalid Artemis API key",
		})
		return
	}

	// Step 2: Construct email if not provided
	email := req.Email
	if email == "" {
		// Use LRZ ID to construct email
		email = req.LrzID + "@mytum.de"
	}

	// Step 3: Get user from database (must already exist)
	// Security: We do NOT auto-create users. Users must be pre-registered in TUMlive
	// and assigned to courses by administrators before they can upload.
	user, err := r.UsersDao.GetUserByEmail(c, email)
	if err != nil {
		// User doesn't exist - reject authentication
		logger.Warn("User not found in TUMlive database", "lrzId", req.LrzID, "email", email)
		c.JSON(http.StatusForbidden, artemisSSOResponse{
			Success: false,
			Error:   "user account not found in TUMlive - please contact administrator to set up your account and course access",
		})
		return
	}

	logger.Info("User authenticated via SSO", "userID", user.ID, "name", user.Name, "lrzId", req.LrzID)

	// Step 4: Check if user has lecturer or admin role (required for uploads)
	if user.Role != model.LecturerType && user.Role != model.AdminType {
		logger.Warn("User lacks upload permissions", "userID", user.ID, "role", user.Role)
		c.JSON(http.StatusForbidden, artemisSSOResponse{
			Success: false,
			Error:   "user does not have permission to upload videos (must be lecturer or admin)",
		})
		return
	}

	// Step 5: Get or create upload token for this user
	token, err := r.getOrCreateUploadTokenSSO(c, user.ID)
	if err != nil {
		logger.Error("Failed to get/create token", "err", err)
		c.JSON(http.StatusInternalServerError, artemisSSOResponse{
			Success: false,
			Error:   "failed to create upload token",
		})
		return
	}

	// Step 6: Get courses where user can upload (is admin + VOD enabled)
	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(c, user.ID, "", 0)
	if err != nil {
		logger.Error("Failed to get courses", "err", err)
		c.JSON(http.StatusInternalServerError, artemisSSOResponse{
			Success: false,
			Error:   "failed to retrieve courses",
		})
		return
	}

	// Filter to only VOD-enabled courses and format for response
	baseURL := "https://" + c.Request.Host
	if c.Request.Header.Get("X-Forwarded-Proto") == "http" || c.Request.TLS == nil {
		baseURL = "http://" + c.Request.Host // Use http for local development
	}

	vodCourses := []courseInfo{}
	for _, course := range courses {
		if course.VODEnabled {
			vodCourses = append(vodCourses, courseInfo{
				ID:          course.ID,
				Name:        course.Name,
				Slug:        course.Slug,
				Year:        course.Year,
				Term:        course.TeachingTerm,
				UploadURL:   fmt.Sprintf("%s/api/course/artemis/%d/upload", baseURL, course.ID),
				Description: fmt.Sprintf("%s (%s %d)", course.Name, course.TeachingTerm, course.Year),
			})
		}
	}

	// Step 7: Build and return successful response
	response := artemisSSOResponse{
		Success: true,
		Token:   token.Token,
		Courses: vodCourses,
		BaseURL: baseURL,
	}

	if token.Expires.Valid {
		response.Expires = token.Expires.Time.Format(time.RFC3339)
	}

	response.User.ID = user.ID
	response.User.Name = user.Name
	if user.Email.Valid {
		response.User.Email = user.Email.String
	}

	// Convert role uint to string
	roleStr := "student"
	switch user.Role {
	case model.AdminType:
		roleStr = "admin"
	case model.LecturerType:
		roleStr = "lecturer"
	case model.GenericType:
		roleStr = "generic"
	case model.StudentType:
		roleStr = "student"
	}
	response.User.Role = roleStr

	logger.Info("Artemis SSO authentication successful",
		"userID", user.ID,
		"lrzId", req.LrzID,
		"coursesAvailable", len(vodCourses),
		"tokenExpires", response.Expires)

	c.JSON(http.StatusOK, response)
}

// getOrCreateUploadTokenSSO gets an existing valid token or creates a new one
func (r artemisSSORoutes) getOrCreateUploadTokenSSO(c *gin.Context, userID uint) (*model.Token, error) {
	// Get user for token lookup
	user, err := r.UsersDao.GetUserByID(c, userID)
	if err != nil {
		return nil, err
	}

	// Check if user already has a valid lecturer token
	allTokens, err := r.TokenDao.GetAllTokens(&user)
	if err != nil {
		return nil, err
	}

	// Look for an active lecturer token that hasn't expired
	for _, tokenDto := range allTokens {
		if tokenDto.Scope == model.TokenScopeLecturer {
			// Check if token is expired
			if tokenDto.Expires.Valid && tokenDto.Expires.Time.Before(time.Now()) {
				continue // Skip expired tokens
			}

			// Found a valid token, return it
			logger.Info("Reusing existing token", "userID", userID, "tokenID", tokenDto.ID)
			token := tokenDto.Token // Extract the embedded model.Token
			return &token, nil
		}
	}

	// No valid token found, create a new one (valid for 1 year)
	tokenStr := uuid.NewV4().String()
	expires := sql.NullTime{
		Valid: true,
		Time:  time.Now().AddDate(1, 0, 0), // 1 year from now
	}

	newToken := model.Token{
		UserID:  userID,
		Token:   tokenStr,
		Expires: expires,
		Scope:   model.TokenScopeLecturer,
	}

	err = r.TokenDao.AddToken(newToken)
	if err != nil {
		return nil, err
	}

	logger.Info("Created new token", "userID", userID, "tokenID", newToken.ID, "expires", expires.Time)
	return &newToken, nil
}
