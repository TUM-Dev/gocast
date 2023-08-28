package web

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"strings"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	log "github.com/sirupsen/logrus"
)

func configSaml(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	// don't configure saml if no config is set
	if tools.Cfg.Saml == nil {
		return
	}

	// create saml.ServiceProvider
	keyPair, err := tls.LoadX509KeyPair(tools.Cfg.Saml.Cert, tools.Cfg.Saml.Privkey)
	if err != nil {
		log.WithError(err).Fatal("Could not load SAML keypair")
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		log.WithError(err).Fatal("Could not parse SAML keypair")
	}
	idpMetadataURL, err := url.Parse(tools.Cfg.Saml.IdpMetadataURL)
	if err != nil {
		log.WithError(err).Fatal("Could not parse Identity Provider metadata URL")
	}
	idpMetadata, err := samlsp.FetchMetadata(context.Background(), http.DefaultClient,
		*idpMetadataURL)
	if err != nil {
		log.WithError(err).Error("Could not load Identity Provider metadata")
	}

	var samlSPs []*samlsp.Middleware
	for _, l := range tools.Cfg.Saml.RootURLs {
		u, err := url.Parse(l)
		if err != nil {
			log.WithError(err).Error("Could not parse Root URL")
			continue
		}
		samlSP, err := samlsp.New(samlsp.Options{
			URL:               *u,
			Key:               keyPair.PrivateKey.(*rsa.PrivateKey),
			Certificate:       keyPair.Leaf,
			IDPMetadata:       idpMetadata,
			EntityID:          tools.Cfg.Saml.EntityID,
			AllowIDPInitiated: true,
		})
		if err != nil {
			log.WithError(err).Fatal("Could not create SAML Service Provider")
		}
		samlSP.ServiceProvider.AcsURL = *u
		samlSPs = append(samlSPs, samlSP)
	}

	// serve metadata. This can be fetched periodically by the IDP.
	r.GET("/saml/metadata", func(c *gin.Context) {
		getSamlSpFromHost(samlSPs, c.Request.Host).ServeMetadata(c.Writer, c.Request)
	})

	// /saml/out is accessed to login with the IDP.
	// It will redirect to http://login.idp.something/... which will redirect back to us on success.
	r.GET("/saml/out", func(c *gin.Context) {
		getSamlSpFromHost(samlSPs, c.Request.Host).HandleStartAuthFlow(c.Writer, c.Request)
	})

	// /saml/slo is accessed after the IDP logged out the user.
	r.POST("/saml/slo", func(c *gin.Context) {
		err := c.Request.ParseForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		err = getSamlSpFromHost(samlSPs, c.Request.Host).ServiceProvider.ValidateLogoutResponseForm(c.Request.PostFormValue("SAMLResponse"))
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": "403- Forbidden", "error": "Invalid logout data: " + err.Error()})
			return
		}
		c.SetCookie("jwt", "", -1, "/", "", tools.CookieSecure, true)
		c.Redirect(http.StatusFound, "/")
	})

	// /saml/logout redirects to the idp with a logout request.
	// The idp will redirect back to /saml/slo after the user logged out.
	r.GET("/saml/logout", func(c *gin.Context) {
		foundContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
		if foundContext.SamlSubjectID != nil {
			request, err := getSamlSpFromHost(samlSPs, c.Request.Host).ServiceProvider.MakeRedirectLogoutRequest(*foundContext.SamlSubjectID, "")
			if err != nil {
				return
			}
			log.Info("Logout request: ", request)
			c.Redirect(http.StatusFound, request.String())
		}
	})

	// /shib is accessed after authentication with the IDP. The post body contains the encrypted SAMLResponse.
	r.POST("/shib", func(c *gin.Context) {
		err := c.Request.ParseForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "400 - Bad Request", "error": err.Error()})
		}
		response, err := getSamlSpFromHost(samlSPs, c.Request.Host).ServiceProvider.ParseResponse(c.Request, []string{""})
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": "403- Forbidden", "error": err.Error()})
			return
		}

		lrzID := extractSamlField(response, "uid")
		matrNr := extractSamlField(response, "imMatrikelNr")
		firstName := extractSamlField(response, "givenName")
		lastName := extractSamlField(response, "sn")
		subjectID := extractSamlField(response, "samlSubjectID") // used to logout from the IDP
		var lastNameUser *string
		if lastName != "" {
			lastNameUser = &lastName
		}
		if matrNr == "" {
			matrNr = extractSamlField(response, "eduPersonPrincipalName") // MWN id if no matrNr
			s := strings.Split(matrNr, "@")
			if len(s) == 0 || s[0] == "" {
				log.WithFields(log.Fields{
					"LRZ-ID":    lrzID,
					"firstName": firstName,
					"lastName":  lastName,
					"mwnID":     matrNr,
				}).Error("Can't extract mwn id")
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			matrNr = s[0]
		}
		user := model.User{
			Name:                firstName,
			LastName:            lastNameUser,
			MatriculationNumber: matrNr,
			LrzID:               lrzID,
		}
		err = daoWrapper.UsersDao.UpsertUser(&user)
		if err != nil {
			log.WithError(err).Error("Could not upsert user")
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		HandleValidLogin(c, &tools.SessionData{Userid: user.ID, SamlSubjectID: &subjectID})
	})
}

func getSamlSpFromHost(samlSPs []*samlsp.Middleware, host string) *samlsp.Middleware {
	for _, samlSP := range samlSPs {
		if strings.Contains(samlSP.ServiceProvider.AcsURL.String(), strings.Split(host, ":")[0]) {
			return samlSP
		}
	}
	return samlSPs[0]
}

// extractSamlField gets the value of the given field from the SAML response or an empty string if the field is not present.
func extractSamlField(assertion *saml.Assertion, friendlyFieldName string) string {
	for _, statement := range assertion.AttributeStatements {
		for _, attribute := range statement.Attributes {
			if attribute.FriendlyName == friendlyFieldName && len(attribute.Values) > 0 {
				return attribute.Values[0].Value
			}
		}
	}
	return ""
}
