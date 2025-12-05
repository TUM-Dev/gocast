package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func configSiteSettingsRouter(engine *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := siteSettingsRoutes{daoWrapper}

	// Admin routes for managing settings
	adminGroup := engine.Group("/api/siteSettings")
	adminGroup.Use(tools.Admin)
	adminGroup.GET("/:key", routes.getSetting)
	adminGroup.PUT("/:key", routes.setSetting)

	// Theme management routes (admin only)
	themeGroup := engine.Group("/api/theme")
	themeGroup.Use(tools.Admin)
	themeGroup.GET("/available", routes.getAvailableThemes)
	themeGroup.PUT("/active", routes.setActiveTheme)

	// Public route for getting active theme (used by all pages)
	engine.GET("/api/theme/active", routes.getActiveTheme)
}

type siteSettingsRoutes struct {
	dao.DaoWrapper
}

func (r siteSettingsRoutes) getSetting(c *gin.Context) {
	key := c.Param("key")
	value, err := r.SiteSettingsDao.Get(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"value": ""})
			return
		}
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get setting",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"value": value})
}

type setSettingRequest struct {
	Value string `json:"value"`
}

func (r siteSettingsRoutes) setSetting(c *gin.Context) {
	key := c.Param("key")
	var req setSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	err := r.SiteSettingsDao.Set(key, req.Value)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not set setting",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (r siteSettingsRoutes) getAvailableThemes(c *gin.Context) {
	themes := model.AvailableThemes()
	c.JSON(http.StatusOK, themes)
}

func (r siteSettingsRoutes) getActiveTheme(c *gin.Context) {
	themeID, err := r.SiteSettingsDao.Get(model.SettingActiveTheme)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"themeId": string(model.ThemeDefault)})
			return
		}
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get active theme",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"themeId": themeID})
}

type setActiveThemeRequest struct {
	ThemeID string `json:"themeId"`
}

func (r siteSettingsRoutes) setActiveTheme(c *gin.Context) {
	var req setActiveThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	// Validate theme ID
	validTheme := false
	for _, theme := range model.AvailableThemes() {
		if string(theme.ID) == req.ThemeID {
			validTheme = true
			break
		}
	}
	if !validTheme {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid theme ID",
			Err:           nil,
		})
		return
	}

	err := r.SiteSettingsDao.Set(model.SettingActiveTheme, req.ThemeID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not set active theme",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
