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

func configGinSchoolsRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := schoolsRoutes{daoWrapper}

	router.POST("/api/schools/proxy/:token", routes.fetchStreamKey)
	schools := router.Group("/api/schools")
	schools.Use(tools.AdminOrMaintainer)
	{
		schools.GET("/", routes.SearchSchool)
		schools.POST("/", routes.CreateSchool)
		schools.PATCH("/:id", routes.updateSchool)
		schools.DELETE("/:id", routes.DeleteSchool)
		schools.POST("/:id/token", routes.createTokenForSchool)
	}

	router.GET("/api/schools/:id", tools.AtLeastLecturer, routes.GetSchoolById)

	admins := router.Group("/api/schools/:id/admins")
	admins.Use(tools.AdminOrMaintainer)

	{
		admins.GET("/", routes.GetSchoolAdmin)
		admins.POST("/", routes.AddAdminToSchool)
		admins.DELETE("/:admin_id", routes.DeleteAdminFromSchool)
	}

	router.GET("/api/schools/all", tools.AtLeastLecturer, routes.SearchAllSchools)
}

type JWTSchoolClaims struct {
	jwt.RegisteredClaims
	UserID   uint
	SchoolID string
}

func (s *schoolsRoutes) CreateSchool(c *gin.Context) {
	var req createSchoolRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can not bind body"})
		return
	}

	// Check if user is admin
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	// Check if user is maintainer of parent school if parent school is set, otherwise check if user is admin
	var parentIdUint uint
	if req.ParentId != "" {
		parentId, err := strconv.ParseUint(req.ParentId, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parent_id format"})
			return
		}
		parentIdUint = uint(parentId)

		if !s.isAdminOfSchool(c, parentIdUint) {
			c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
			return
		}
	} else if ctx.User.Role != model.AdminType {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins can create schools without parent"})
		return
	}

	// Check if a school with the same name already exists
	if existingSchool, err := s.SchoolsDao.GetByName(c, req.Name); err != nil {
		if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check for existing school"})
			return
		}
	} else if existingSchool.ID != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a school with this name already exists"})
		return
	}

	// parentSchool, err := s.SchoolsDao.Get(c, parentIdUint)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get parent school"})
	// 	return
	// }

	school := model.School{
		Name:     req.Name,
		OrgType:  req.OrgType,
		ParentID: parentIdUint,
	}

	if err := s.SchoolsDao.Create(c, &school); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create school"})
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
	if err := s.SchoolsDao.AddAdmin(c, &school, ctx.User); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not add admin to school"})
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

		if err := s.SchoolsDao.AddAdmin(c, &school, &u); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not add admin to school"})
			return
		}
	}

	c.JSON(http.StatusOK, school)
}

func (s *schoolsRoutes) DeleteSchool(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !s.isAdminOfSchool(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
		return
	}
	if err := s.SchoolsDao.Delete(c, uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete school"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "school deleted"})
}

func (s *schoolsRoutes) SearchSchool(c *gin.Context) {
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	if ctx.User == nil {
		return
	}

	query := c.Query("q")
	schools, err := s.SchoolsDao.QueryAdministerdSchools(c, ctx.User, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search schools"})
		return
	}

	type relevantSchoolInfo struct {
		ID       uint         `json:"id"`
		Name     string       `json:"name"`
		OrgId    string       `json:"org_id"`
		OrgType  string       `json:"orgType"`
		OrgSlug  string       `json:"org_slug"`
		Admins   []model.User `json:"admins"`
		ParentID uint         `json:"parent_id"`
	}

	res := make([]relevantSchoolInfo, len(schools))
	for i, school := range schools {
		res[i] = relevantSchoolInfo{
			ID:       school.ID,
			Name:     school.Name,
			OrgId:    school.OrgId,
			OrgType:  school.OrgType,
			OrgSlug:  school.OrgSlug,
			Admins:   school.Admins,
			ParentID: school.ParentID,
		}
	}
	c.JSON(http.StatusOK, res)
}

func (s schoolsRoutes) SearchAllSchools(c *gin.Context) {
	query := c.Query("q")
	schools, err := s.SchoolsDao.Query(c, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search schools"})
		return
	}

	type relevantSchoolInfo struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		OrgType string `json:"org_type"`
	}

	res := make([]relevantSchoolInfo, len(schools))
	for i, school := range schools {
		res[i] = relevantSchoolInfo{
			ID:      school.ID,
			Name:    school.Name,
			OrgType: school.OrgType,
		}
	}
	c.JSON(http.StatusOK, res)
}

func (s *schoolsRoutes) updateSchool(c *gin.Context) {
	var req updateSchoolRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can not bind body"})
		return
	}

	if !s.isAdminOfSchool(c, req.Id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
		return
	}

	school := model.School{
		Model:   gorm.Model{ID: req.Id},
		Name:    req.Name,
		OrgType: req.OrgType,
	}

	if err := s.SchoolsDao.Update(c, &school); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update school"})
		return
	}

	c.JSON(http.StatusOK, school)
}

func (s *schoolsRoutes) GetSchoolAdmin(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	admins, err := s.SchoolsDao.GetAdmins(c, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get admins"})
		return
	}

	c.JSON(http.StatusOK, admins)
}

func (s *schoolsRoutes) AddAdminToSchool(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !s.isAdminOfSchool(c, uint(id)) {
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

	if err := s.SchoolsDao.AddAdmin(c, &model.School{Model: gorm.Model{ID: uint(id)}}, &u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not add admin to school"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "admin added to school"})
}

func (s *schoolsRoutes) DeleteAdminFromSchool(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminIdStr := c.Param("admin_id")
	if adminIdStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin_id parameter is required"})
		return
	}
	adminId, _ := strconv.Atoi(adminIdStr)
	if !s.isAdminOfSchool(c, uint(id)) {
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
		c.JSON(http.StatusForbidden, gin.H{"error": "you can't remove super-admins from a school"})
		return
	}

	// Check if the current user is the only admin left
	adminCount, err := s.SchoolsDao.GetAdminCount(c, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch admin count"})
		return
	}

	if adminCount <= 1 && admin.ID == ctx.User.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can't remove yourself as the only maintainer"})
		return
	}

	if err := s.SchoolsDao.RemoveAdmin(c, uint(id), uint(adminId)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not remove admin from school"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "admin removed from school"})
}

func (s *schoolsRoutes) isAdminOfSchool(c *gin.Context, schoolId uint) bool {
	admins, err := s.SchoolsDao.GetAdmins(c, schoolId)
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

func (s *schoolsRoutes) GetSchoolById(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	school, err := s.SchoolsDao.Get(c, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get school"})
		return
	}

	c.JSON(http.StatusOK, school)
}

// This is used by the proxy to get the stream key of the next stream of the lecturer given a lecturer token
//
//	Proxy receives: rtmp://proxy.example.com/<lecturer-token>
//				or: rtmp://proxy.example.com/<lecturer-token>?slug=ABC-123 <-- optional slug parameter in case the lecturer is streaming multiple courses simultaneously
//
//	Proxy returns:  rtmp://ingest.example.com/ABC-123?secret=610f609e4a2c43ac8a6d648177472b17
func (s *schoolsRoutes) fetchStreamKey(c *gin.Context) {
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

	// Find current/next stream and course of which the user is a lecturer
	year, term := tum.GetCurrentSemester()
	courseID, streamKey, courseSlug, err := s.StreamsDao.GetSoonStartingStreamInfo(token.UserID, slug, year, term)
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

	// Get school to redirect to the dedicated ingest server of the school
	school, err := s.SchoolsDao.Get(c, course.SchoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get school"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": "" + school.IngestServerURL + "/" + courseSlug + "?secret=" + streamKey + "/" + courseSlug})
}

func (s *schoolsRoutes) createTokenForSchool(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !s.isAdminOfSchool(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "operation not allowed"})
		return
	}
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	token := jwt.New(jwt.GetSigningMethod("RS256"))
	token.Claims = &JWTSchoolClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 7)}, // Token expires in 7 hours
		},
		UserID:   ctx.User.ID,
		SchoolID: strconv.FormatUint(uint64(id), 10),
	}
	str, err := token.SignedString(tools.Cfg.GetJWTKey())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create school token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": str})
}

type schoolsRoutes struct {
	dao.DaoWrapper
}

type updateSchoolRequest struct {
	Id      uint   `json:"id"`
	Name    string `json:"name"`
	OrgType string `json:"org_type"`
}

type createSchoolRequest struct {
	AdminEmail string `json:"admin_email"`
	Name       string `json:"name"`
	OrgType    string `json:"org_type"`
	ParentId   string `json:"parent_id"`
}
