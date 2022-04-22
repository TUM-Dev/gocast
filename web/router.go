package web

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"fmt"
	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"html/template"
	"net/http"
	"net/url"
	"strings"
)

var templ *template.Template

//go:embed template
var templateFS embed.FS

//go:embed assets/*
//go:embed node_modules
var staticFS embed.FS

func ConfigGinRouter(router *gin.Engine) {
	templ = template.Must(template.ParseFS(templateFS,
		"template/*.gohtml",
		"template/admin/*.gohtml",
		"template/admin/admin_tabs/*.gohtml",
		"template/partial/*.gohtml",
		"template/partial/stream/*.gohtml",
		"template/partial/course/manage/*.gohtml",
		"template/partial/admin/*.gohtml",
		"template/partial/stream/chat/*.gohtml",
		"template/partial/course/manage/*.gohtml"))
	tools.SetTemplates(templ)
	configGinStaticRouter(router)
	configSaml(router)
	configMainRoute(router)
	configCourseRoute(router)
}

func configSaml(r *gin.Engine) {
	if tools.Cfg.Saml == nil {
		return
	}
	keyPair, err := tls.LoadX509KeyPair(tools.Cfg.Saml.Cert, tools.Cfg.Saml.Privkey)
	if err != nil {
		panic(err) // TODO handle error
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		panic(err) // TODO handle error
	}

	idpMetadataURL, err := url.Parse(tools.Cfg.Saml.IdpMetadataURL)
	if err != nil {
		panic(err) // TODO handle error
	}
	idpMetadata, err := samlsp.FetchMetadata(context.Background(), http.DefaultClient,
		*idpMetadataURL)
	if err != nil {
		panic(err) // TODO handle error
	}

	rootURL, err := url.Parse(tools.Cfg.Saml.RootURL)
	if err != nil {
		panic(err) // TODO handle error
	}

	samlSP, err := samlsp.New(samlsp.Options{
		URL:               *rootURL,
		Key:               keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate:       keyPair.Leaf,
		IDPMetadata:       idpMetadata,
		EntityID:          tools.Cfg.Saml.EntityID,
		AllowIDPInitiated: true,
	})
	samlSP.ServiceProvider.AcsURL = *rootURL
	if err != nil {
		panic(err)
	}
	r.GET("/saml/metadata", func(c *gin.Context) {
		samlSP.ServeMetadata(c.Writer, c.Request)
	})
	r.Any("/saml/acs", func(c *gin.Context) {
		samlSP.ServeACS(c.Writer, c.Request)
	})
	r.Any("/shib", func(c *gin.Context) {
		err := c.Request.ParseForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "400 - Bad Request", "error": err.Error()})
		}
		response, err := samlSP.ServiceProvider.ParseResponse(c.Request, []string{""})
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": "403- Forbidden", "error": err.Error()})
			return
		}
		for _, statement := range response.AttributeStatements {
			for _, attribute := range statement.Attributes {
				if attribute.FriendlyName == "displayName" {
					if len(attribute.Values) > 0 {
						c.String(http.StatusOK, "Hi, "+attribute.Values[0].Value+"!")
						return
					}
				}
			}
		}
	})
}

func configGinStaticRouter(router gin.IRoutes) {
	router.Static("/public", tools.Cfg.Paths.Static)
	router.StaticFS("/static", http.FS(staticFS))
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.FileFromFS("assets/favicon.ico", http.FS(staticFS))
	})
}

func formatRequest(r *http.Request) string {
	// Create return string
	var request []string // Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)                             // Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host)) // Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	} // Return the request as a string
	return strings.Join(request, "\n")
}

//todo: un-export functions
func configMainRoute(router *gin.Engine) {
	streamGroup := router.Group("/")

	atLeastLecturerGroup := router.Group("/")
	atLeastLecturerGroup.Use(tools.AtLeastLecturer)
	atLeastLecturerGroup.GET("/admin", AdminPage)
	atLeastLecturerGroup.GET("/admin/create-course", AdminPage)
	router.GET("/about", AboutPage)

	adminGroup := router.Group("/")
	adminGroup.GET("/admin/users", AdminPage)
	adminGroup.GET("/admin/lectureHalls", AdminPage)
	adminGroup.GET("/admin/lectureHalls/new", AdminPage)
	adminGroup.GET("/admin/workers", AdminPage)
	adminGroup.GET("/admin/server-notifications", AdminPage)
	adminGroup.GET("/admin/server-stats", AdminPage)
	adminGroup.GET("/admin/course-import", AdminPage)
	adminGroup.GET("/admin/token", AdminPage)
	adminGroup.GET("/admin/notifications", AdminPage)

	courseAdminGroup := router.Group("/")
	courseAdminGroup.Use(tools.InitCourse)
	courseAdminGroup.Use(tools.AdminOfCourse)
	courseAdminGroup.GET("/admin/course/:courseID", EditCoursePage)
	courseAdminGroup.GET("/admin/course/:courseID/stats", CourseStatsPage)
	courseAdminGroup.POST("/admin/course/:courseID", UpdateCourse)

	withStream := courseAdminGroup.Group("/")
	withStream.Use(tools.InitStream)
	withStream.GET("/admin/units/:courseID/:streamID", LectureUnitsPage)
	withStream.GET("/admin/cut/:courseID/:streamID", LectureCutPage)

	router.POST("/login", LoginHandler)
	router.GET("/login", LoginPage)
	router.GET("/logout", LogoutPage)
	router.GET("/setPassword/:key", CreatePasswordPage)
	router.POST("/setPassword/:key", CreatePasswordPage)
	streamGroup.Use(tools.InitStream)
	streamGroup.GET("/w/:slug/:streamID", WatchPage)
	streamGroup.GET("/w/:slug/:streamID/:version", WatchPage)
	streamGroup.GET("/w/:slug/:streamID/chat/popup", PopUpChat)
	router.GET("/", MainPage)
	router.GET("/semester/:year/:term", MainPage)
	router.GET("/healthcheck", HealthCheck)

	router.GET("/:shortLink", HighlightPage)
	router.GET("/edit-course", editCourseByTokenPage)

	// redirect from old site:
	router.GET("/cgi-bin/streams/*x", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})
	router.NoRoute(func(c *gin.Context) {
		tools.RenderErrorPage(c, http.StatusNotFound, tools.PageNotFoundErrMsg)
	})
}

func configCourseRoute(router *gin.Engine) {
	g := router.Group("/course")
	g.Use(tools.InitCourse)
	g.GET("/:year/:teachingTerm/:slug", CoursePage)
}

func HealthCheck(context *gin.Context) {
	resp := HealthCheckData{
		Version:      VersionTag,
		CacheMetrics: CacheMetrics{Hits: dao.Cache.Metrics.Hits(), Misses: dao.Cache.Metrics.Misses(), KeysAdded: dao.Cache.Metrics.KeysAdded()},
	}
	context.JSON(http.StatusOK, resp)
}

type HealthCheckData struct {
	Version      string       `json:"version"`
	CacheMetrics CacheMetrics `json:"cacheMetrics"`
}
type CacheMetrics struct {
	Hits      uint64 `json:"hits"`
	Misses    uint64 `json:"misses"`
	KeysAdded uint64 `json:"keysAdded"`
}

type ChatData struct {
	IsAdminOfCourse bool // is current user admin or lecturer who created the course associated with the chat
	IndexData       IndexData
	IsPopUp         bool
}
