package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
)

func configGinSchoolsRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := schoolsRoutes{daoWrapper}

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

	// Check if the school name is reserved
	if req.Name == "master" || req.University == "service" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reserved school name or university"})
		return
	}

	// Check if a school with the same name and university already exists
	if existingSchool, err := s.SchoolsDao.GetByNameAndUniversity(c, req.Name, req.University); err != nil {
		if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check for existing school"})
			return
		}
	} else if existingSchool.ID != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a school with this name and university already exists"})
		return
	}

	school := model.School{
		Name:       req.Name,
		University: req.University,
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
		ID         uint         `json:"id"`
		Name       string       `json:"name"`
		OrgId      string       `json:"org_id"`
		OrgType    string       `json:"org_type"`
		OrgSlug    string       `json:"org_slug"`
		University string       `json:"university"`
		Admins     []model.User `json:"admins"`
	}

	res := make([]relevantSchoolInfo, len(schools))
	for i, school := range schools {
		res[i] = relevantSchoolInfo{
			ID:         school.ID,
			Name:       school.Name,
			OrgId:      school.OrgId,
			OrgType:    school.OrgType,
			OrgSlug:    school.OrgSlug,
			University: school.University,
			Admins:     school.Admins,
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
		ID         uint   `json:"id"`
		Name       string `json:"name"`
		University string `json:"university"`
	}

	res := make([]relevantSchoolInfo, len(schools))
	for i, school := range schools {
		res[i] = relevantSchoolInfo{
			ID:         school.ID,
			Name:       school.Name,
			University: school.University,
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
		Model:      gorm.Model{ID: req.Id},
		Name:       req.Name,
		University: req.University,
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
	Id         uint   `json:"id"`
	Name       string `json:"name"`
	University string `json:"university"`
}

type createSchoolRequest struct {
	AdminEmail string `json:"admin_email"`
	Name       string `json:"name"`
	University string `json:"university"`
}
