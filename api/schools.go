package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func configGinSchoolsRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := schoolsRoutes{daoWrapper}

	resources := router.Group("/api/schools/:id/resources")
	resources.Use(tools.AdminOrMaintainer)

	{
		resources.GET("/:resource_id", routes.getResourceForSchool)
		resources.POST("/resource", routes.postResource)
		resources.PATCH("/:resource_id", routes.updateResource)
		resources.DELETE("/resource", routes.deleteResource)
	}
	schools := router.Group("/api/schools")
	schools.Use(tools.AdminOrMaintainer)
	{
		schools.GET("/", routes.SearchSchool)
		schools.POST("/", routes.CreateSchool)
		schools.PATCH("/:id", routes.updateSchool)
		schools.DELETE("/:id", routes.DeleteSchool)
	}

	admins := router.Group("/api/schools/:id/admins")
	admins.Use(tools.AdminOrMaintainer)

	{
		admins.GET("/", routes.SearchForSchoolAdmin)
		admins.POST("/", routes.AddAdminToSchool)
		admins.DELETE("/:admin_id", routes.DeleteAdminFromSchool)
	}

}

func (s *schoolsRoutes) getResourceForSchool(c *gin.Context) {
	// TODO: Implement this function
}

func (s *schoolsRoutes) postResource(c *gin.Context) {
	// TODO: Implement this function
}

func (s *schoolsRoutes) updateResource(c *gin.Context) {
	// TODO: Implement this function
}

func (s *schoolsRoutes) deleteResource(c *gin.Context) {
	// TODO: Implement this function
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
		Name:                   req.Name,
		University:             req.University,
		SharedResourcesAllowed: req.SharedResourcesAllowed,
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
		ID                     uint         `json:"id"`
		Name                   string       `json:"name"`
		University             string       `json:"university"`
		SharedResourcesAllowed bool         `json:"shared_resources_allowed"`
		Admins                 []model.User `json:"admins"`
	}

	res := make([]relevantSchoolInfo, len(schools))
	for i, school := range schools {
		res[i] = relevantSchoolInfo{
			ID:                     school.ID,
			Name:                   school.Name,
			University:             school.University,
			SharedResourcesAllowed: school.SharedResourcesAllowed,
			Admins:                 school.Admins,
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
		Model:                  gorm.Model{ID: req.Id},
		Name:                   req.Name,
		University:             req.University,
		SharedResourcesAllowed: req.SharedResourcesAllowed,
	}

	if err := s.SchoolsDao.Update(c, &school); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update school"})
		return
	}

	c.JSON(http.StatusOK, school)
}

func (s *schoolsRoutes) SearchForSchoolAdmin(c *gin.Context) {
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
		Email string `json:"email"`
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

	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
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
	for _, admin := range admins {
		if admin.ID == ctx.User.ID {
			return true
		}
	}

	return false
}

type schoolsRoutes struct {
	dao.DaoWrapper
}

type schoolResponse struct {
	SchoolData struct {
		Name                   string    `json:"name,omitempty"`
		University             string    `json:"university,omitempty"`
		SharedResourcesAllowed bool      `json:"shared_resources_allowed,omitempty"`
		CreatedAt              time.Time `json:"created_at"`
	} `json:"school_data,omitempty"`
	Admins []struct {
		Name     string  `json:"name,omitempty"`
		LastName *string `json:"last_name,omitempty"`
		Email    string  `json:"email,omitempty"`
		Role     uint    `json:"role,omitempty"`
	} `json:"admins,omitempty"`
	Resources []struct {
		ResourceID uint `json:"resource_id"`
	} `json:"resources,omitempty"`
}

type updateSchoolRequest struct {
	Id                     uint   `json:"id"`
	Name                   string `json:"name"`
	University             string `json:"university"`
	SharedResourcesAllowed bool   `json:"shared_resources_allowed"`
}

type createSchoolRequest struct {
	AdminEmail             string `json:"admin_email"`
	Name                   string `json:"name"`
	University             string `json:"university"`
	SharedResourcesAllowed bool   `json:"shared_resources_allowed"`
}

type createSchoolResponse struct {
	Name                   string `json:"name"`
	University             string `json:"university"`
	SharedResourcesAllowed bool   `json:"shared_resources_allowed"`
}
