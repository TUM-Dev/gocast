package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
)

func configGinOrganizationsRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := organizationsRoutes{daoWrapper}

	router.POST("/api/organizations/proxy/:token", routes.fetchStreamKey)
	organizations := router.Group("/api/organizations")
	organizations.Use(tools.AdminOrMaintainer)
	{
		organizations.GET("/", routes.SearchOrganization)
		organizations.POST("/", routes.CreateOrganization)
		organizations.PATCH("/:id", routes.updateOrganization)
		organizations.DELETE("/:id", routes.DeleteOrganization)
		organizations.POST("/:id/token", routes.createTokenForOrganization)
	}

	router.GET("/api/organizations/:id", tools.AtLeastLecturer, routes.GetOrganizationById)

	admins := router.Group("/api/organizations/:id/admins")
	admins.Use(tools.AdminOrMaintainer)

	{
		admins.GET("/", routes.GetOrganizationAdmin)
		admins.POST("/", routes.AddAdminToOrganization)
		admins.DELETE("/:admin_id", routes.DeleteAdminFromOrganization)
	}

	router.GET("/api/organizations/all", tools.AtLeastLecturer, routes.SearchAllOrganizations)
}

type JWTOrganizationClaims struct {
	jwt.RegisteredClaims
	UserID         uint
	OrganizationID string
}

func (s *organizationsRoutes) CreateOrganization(c *gin.Context) {
	var req createOrganizationRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can not bind body"})
		return
	}

	// Check if user is admin
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	// Check if user is maintainer of parent organization if parent organization is set, otherwise check if user is admin
	var parentIdUint uint
	if req.ParentId != "" {
		parentId, err := strconv.ParseUint(req.ParentId, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parent_id format"})
			return
		}
		parentIdUint = uint(parentId)

		if !s.isAdminOfOrganization(c, parentIdUint) {
			c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
			return
		}
	} else if ctx.User.Role != model.AdminType {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins can create organizations without parent"})
		return
	}

	// Check if a organization with the same name already exists
	if existingOrganization, err := s.OrganizationsDao.GetByName(c, req.Name); err != nil {
		if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check for existing organization"})
			return
		}
	} else if existingOrganization.ID != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a organization with this name already exists"})
		return
	}

	// parentOrganization, err := s.OrganizationsDao.Get(c, parentIdUint)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get parent organization"})
	// 	return
	// }

	organization := model.Organization{
		Name:     req.Name,
		OrgType:  req.OrgType,
		ParentID: parentIdUint,
	}

	if err := s.OrganizationsDao.Create(c, &organization); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create organization"})
		return
	}

	// Add current user as admin
	if ctx.User.Role != model.MaintainerType && ctx.User.Role != model.AdminType {
		ctx.User.Role = model.MaintainerType
		if err := s.UsersDao.UpdateUser(*ctx.User); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update user role"})
			return
		}
	}
	if err := s.OrganizationsDao.AddAdmin(c, &organization, ctx.User); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not add admin to organization"})
		return
	}

	// Add external admin (optional)
	if req.AdminEmail != "" {
		u, err := s.UsersDao.GetUserByEmail(c, req.AdminEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get user"})
			return
		}

		if u.Role != model.MaintainerType && u.Role != model.AdminType {
			u.Role = model.MaintainerType
			if err := s.UsersDao.UpdateUser(u); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update user role"})
				return
			}
		}

		if err := s.OrganizationsDao.AddAdmin(c, &organization, &u); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not add admin to organization"})
			return
		}
	}

	c.JSON(http.StatusOK, organization)
}

func (s *organizationsRoutes) DeleteOrganization(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !s.isAdminOfOrganization(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
		return
	}
	if err := s.OrganizationsDao.Delete(c, uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete organization"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "organization deleted"})
}

func (s *organizationsRoutes) SearchOrganization(c *gin.Context) {
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	if ctx.User == nil {
		return
	}

	query := c.Query("q")
	organizations, err := s.OrganizationsDao.QueryAdministerdOrganizations(c, ctx.User, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search organizations"})
		return
	}

	type relevantOrganizationInfo struct {
		ID       uint         `json:"id"`
		Name     string       `json:"name"`
		OrgId    string       `json:"org_id"`
		OrgType  string       `json:"orgType"`
		OrgSlug  string       `json:"org_slug"`
		Admins   []model.User `json:"admins"`
		ParentID uint         `json:"parent_id"`
	}

	res := make([]relevantOrganizationInfo, len(organizations))
	for i, organization := range organizations {
		res[i] = relevantOrganizationInfo{
			ID:       organization.ID,
			Name:     organization.Name,
			OrgId:    organization.OrgId,
			OrgType:  organization.OrgType,
			OrgSlug:  organization.OrgSlug,
			Admins:   organization.Admins,
			ParentID: organization.ParentID,
		}
	}
	c.JSON(http.StatusOK, res)
}

func (s organizationsRoutes) SearchAllOrganizations(c *gin.Context) {
	query := c.Query("q")
	organizations, err := s.OrganizationsDao.Query(c, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search organizations"})
		return
	}

	type relevantOrganizationInfo struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		OrgType string `json:"org_type"`
	}

	res := make([]relevantOrganizationInfo, len(organizations))
	for i, organization := range organizations {
		res[i] = relevantOrganizationInfo{
			ID:      organization.ID,
			Name:    organization.Name,
			OrgType: organization.OrgType,
		}
	}
	c.JSON(http.StatusOK, res)
}

func (s *organizationsRoutes) updateOrganization(c *gin.Context) {
	var req updateOrganizationRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can not bind body"})
		return
	}

	if !s.isAdminOfOrganization(c, req.Id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
		return
	}

	organization := model.Organization{
		Model:   gorm.Model{ID: req.Id},
		Name:    req.Name,
		OrgType: req.OrgType,
	}

	if err := s.OrganizationsDao.Update(c, &organization); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update organization"})
		return
	}

	c.JSON(http.StatusOK, organization)
}

func (s *organizationsRoutes) GetOrganizationAdmin(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	admins, err := s.OrganizationsDao.GetAdmins(c, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get admins"})
		return
	}

	c.JSON(http.StatusOK, admins)
}

func (s *organizationsRoutes) AddAdminToOrganization(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !s.isAdminOfOrganization(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
		return
	}

	var req struct {
		Email  string `json:"email"`
		MatrNr string `json:"matr_nr"`
		ID     string `json:"id"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can not bind body"})
		return
	}

	u, err := s.UsersDao.GetUserByEmail(c, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get user"})
		return
	}

	if err := s.OrganizationsDao.AddAdmin(c, &model.Organization{Model: gorm.Model{ID: uint(id)}}, &u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not add admin to organization"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "admin added to organization"})
}

func (s *organizationsRoutes) DeleteAdminFromOrganization(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminIdStr := c.Param("admin_id")
	if adminIdStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin_id parameter is required"})
		return
	}
	adminId, _ := strconv.Atoi(adminIdStr)
	if !s.isAdminOfOrganization(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
		return
	}

	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	// Fetch the user with the adminId
	admin, err := s.GetUserByID(c, uint(adminId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch admin"})
		return
	}

	// Check the role of the admin
	if admin.Role == model.AdminType {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can't remove super-admins from a organization"})
		return
	}

	// Check if the current user is the only admin left
	adminCount, err := s.OrganizationsDao.GetAdminCount(c, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch admin count"})
		return
	}

	if adminCount <= 1 && admin.ID == ctx.User.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can't remove yourself as the only maintainer"})
		return
	}

	if err := s.OrganizationsDao.RemoveAdmin(c, uint(id), uint(adminId)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not remove admin from organization"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "admin removed from organization"})
}

func (s *organizationsRoutes) isAdminOfOrganization(c *gin.Context, organizationId uint) bool {
	admins, err := s.OrganizationsDao.GetAdmins(c, organizationId)
	if err != nil {
		return false
	}

	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	if ctx.User == nil {
		return false
	}
	if ctx.User.Role == model.AdminType {
		return true
	}
	for _, admin := range admins {
		if admin.ID == ctx.User.ID {
			return true
		}
	}

	return false
}

func (s *organizationsRoutes) GetOrganizationById(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	organization, err := s.OrganizationsDao.Get(c, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get organization"})
		return
	}

	c.JSON(http.StatusOK, organization)
}

// This is used by the proxy to get the stream key of the next stream of the lecturer given a lecturer token
//
//	Proxy receives: rtmp://proxy.example.com/<lecturer-token>
//				or: rtmp://proxy.example.com/<lecturer-token>?slug=ABC-123 <-- optional slug parameter in case the lecturer is streaming multiple courses simultaneously
//
//	Proxy returns:  rtmp://ingest.example.com/ABC-123?secret=610f609e4a2c43ac8a6d648177472b17
func (s *organizationsRoutes) fetchStreamKey(c *gin.Context) {
	// Optional slug parameter to get the stream key of a specific course (in case the lecturer is streaming multiple courses simultaneously)
	slug := c.Query("slug")
	t := c.Param("token")

	// Get user from token
	token, err := s.TokenDao.GetToken(t)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid token",
		})
		return
	}

	// Only tokens of type lecturer are allowed to start streaming
	if token.Scope != model.TokenScopeLecturer {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "invalid scope",
		})
		return
	}

	// Get user and check if he has the right to start a stream
	user, err := s.UsersDao.GetUserByID(c, token.UserID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not get user",
			Err:           err,
		})
		return

	}
	if user.Role != model.LecturerType && user.Role != model.AdminType && user.Role != model.MaintainerType {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "user is not a lecturer, maintainer or admin",
		})
		return
	}

	// Find current/next stream and course of which the user is a lecturer
	year, term := tum.GetCurrentSemester()
	courseID, streamKey, courseSlug, err := s.StreamsDao.GetSoonStartingStreamInfo(&user, slug, year, term)
	if err != nil || streamKey == "" || courseSlug == "" {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "no stream found",
			Err:           err,
		})
		return
	}
	course, err := s.CoursesDao.GetCourseById(c, courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get course"})
		return
	}

	// Get organization to redirect to the dedicated ingest server of the organization
	organization, err := s.OrganizationsDao.Get(c, course.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get organization"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": "" + organization.IngestServerURL + "/" + courseSlug + "?secret=" + streamKey + "/" + courseSlug})
}

func (s *organizationsRoutes) createTokenForOrganization(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !s.isAdminOfOrganization(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
		return
	}
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	token := jwt.New(jwt.GetSigningMethod("RS256"))
	token.Claims = &JWTOrganizationClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 7)}, // Token expires in 7 hours
		},
		UserID:         ctx.User.ID,
		OrganizationID: strconv.FormatUint(uint64(id), 10),
	}
	str, err := token.SignedString(tools.Cfg.GetJWTKey())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create organization token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": str})
}

type organizationsRoutes struct {
	dao.DaoWrapper
}

type updateOrganizationRequest struct {
	Id      uint   `json:"id"`
	Name    string `json:"name"`
	OrgType string `json:"org_type"`
}

type createOrganizationRequest struct {
	AdminEmail string `json:"admin_email"`
	Name       string `json:"name"`
	OrgType    string `json:"org_type"`
	ParentId   string `json:"parent_id"`
}
