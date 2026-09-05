package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func configIntegrationRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := integrationRoutes{daoWrapper}
	g := r.Group("/api/integrations")
	g.Use(tools.RequirePermission(model.PermAdministerServer))
	g.POST("", routes.createIntegration)
	g.POST("/:id/key", routes.rotateIntegrationKey)
	g.DELETE("/:id/key", routes.revokeIntegrationKey)
}

type integrationRoutes struct {
	dao.DaoWrapper
}

type createIntegrationRequest struct {
	Name      string `json:"name"`
	ReturnURL string `json:"returnUrl"`
}

func newIntegrationKey() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hash[:], nil
}

func validIntegrationReturnURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	return u.Scheme == "http" && (host == "localhost" || strings.HasSuffix(host, ".localhost") || ip != nil && ip.IsLoopback())
}

func (r integrationRoutes) createIntegration(c *gin.Context) {
	var req createIntegrationRequest
	if err := c.BindJSON(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ReturnURL = strings.TrimSpace(req.ReturnURL)
	if req.Name == "" || len(req.Name) > 100 || len(req.ReturnURL) > 2048 || !validIntegrationReturnURL(req.ReturnURL) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Enter a name and an HTTPS return URL. HTTP is allowed for loopback development.",
		})
		return
	}
	key, hash, err := newIntegrationKey()
	if err != nil {
		logger.Error("could not generate integration API key", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Could not generate an API key.",
			Err:           err,
		})
		return
	}
	integration := model.Integration{Name: req.Name, ReturnURL: req.ReturnURL, APIKeyHash: hash}
	if err := r.IntegrationDao.CreateIntegration(&integration); err != nil {
		logger.Error("could not create integration", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Could not create the integration.",
			Err:           err,
		})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, gin.H{
		"id":        integration.ID,
		"name":      integration.Name,
		"returnUrl": integration.ReturnURL,
		"apiKey":    key,
	})
}

func (r integrationRoutes) rotateIntegrationKey(c *gin.Context) {
	id, ok := integrationID(c)
	if !ok {
		return
	}
	key, hash, err := newIntegrationKey()
	if err != nil {
		logger.Error("could not generate integration API key", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Could not generate an API key.",
			Err:           err,
		})
		return
	}
	if !r.setIntegrationKey(c, id, hash) {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"apiKey": key})
}

func (r integrationRoutes) revokeIntegrationKey(c *gin.Context) {
	id, ok := integrationID(c)
	if !ok || !r.setIntegrationKey(c, id, nil) {
		return
	}
	c.Status(http.StatusNoContent)
}

func integrationID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Invalid integration.",
		})
		return 0, false
	}
	return uint(id), true
}

func (r integrationRoutes) setIntegrationKey(c *gin.Context, id uint, hash []byte) bool {
	if err := r.IntegrationDao.SetIntegrationAPIKey(id, hash); err != nil {
		status := http.StatusInternalServerError
		message := "Could not update the API key."
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
			message = "Integration not found."
		} else {
			logger.Error("could not update integration API key", "err", err)
		}
		_ = c.Error(tools.RequestError{Status: status, CustomMessage: message, Err: err})
		return false
	}
	return true
}
