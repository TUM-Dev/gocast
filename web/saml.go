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

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func configSaml(r *tools.Router, daoWrapper dao.DaoWrapper) {
	// don't configure saml if no config is set
	if tools.Cfg.Saml == nil {
		return
	}

	// create saml.ServiceProvider
	keyPair, err := tls.LoadX509KeyPair(tools.Cfg.Saml.Cert, tools.Cfg.Saml.Privkey)
	if err != nil {
		logger.Error("Could not load SAML keypair", "err", err)
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		logger.Error("Could not parse SAML keypair", "err", err)
	}
	idpMetadataURL, err := url.Parse(tools.Cfg.Saml.IdpMetadataURL)
	if err != nil {
		logger.Error("Could not parse Identity Provider metadata URL", "err", err)
	}
	idpMetadata, err := samlsp.FetchMetadata(context.Background(), http.DefaultClient,
		*idpMetadataURL)
	if err != nil {
		logger.Error("Could not load Identity Provider metadata", "err", err)
	}

	var samlSPs []*samlsp.Middleware
	for _, l := range tools.Cfg.Saml.RootURLs {
		u, err := url.Parse(l)
		if err != nil {
			logger.Error("Could not parse Root URL", "err", err)
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
			logger.Error("Could not create SAML Service Provider", "err", err)
		}
		samlSP.ServiceProvider.AcsURL = *u
		samlSPs = append(samlSPs, samlSP)
	}

	// serve metadata. This can be fetched periodically by the IDP.
	r.HandleFunc("GET /saml/metadata", func(w http.ResponseWriter, req *http.Request) {
		getSamlSpFromHost(samlSPs, req.Host).ServeMetadata(w, req)
	})

	// /saml/out is accessed to login with the IDP.
	// It will redirect to http://login.idp.something/... which will redirect back to us on success.
	r.HandleFunc("GET /saml/out", func(w http.ResponseWriter, req *http.Request) {
		getSamlSpFromHost(samlSPs, req.Host).HandleStartAuthFlow(w, req)
	})

	// /saml/slo is accessed after the IDP logged out the user.
	r.HandleFunc("POST /saml/slo", func(w http.ResponseWriter, req *http.Request) {
		err := req.ParseForm()
		if err != nil {
			tools.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		err = getSamlSpFromHost(samlSPs, req.Host).ServiceProvider.ValidateLogoutResponseForm(req.PostFormValue("SAMLResponse"))
		if err != nil {
			tools.WriteJSON(w, http.StatusForbidden, map[string]any{"code": "403- Forbidden", "error": "Invalid logout data: " + err.Error()})
			return
		}
		tools.SetCookie(w, "jwt", "", -1, tools.CookieSecure)
		http.Redirect(w, req, "/", http.StatusFound)
	})

	// /saml/logout redirects to the idp with a logout request.
	// The idp will redirect back to /saml/slo after the user logged out.
	r.HandleFunc("GET /saml/logout", func(w http.ResponseWriter, req *http.Request) {
		foundContext, ok := tools.GetTUMLiveContext(req)
		if !ok {
			logger.Error("context should exist but doesn't")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if foundContext.SamlSubjectID != nil {
			request, err := getSamlSpFromHost(samlSPs, req.Host).ServiceProvider.MakeRedirectLogoutRequest(*foundContext.SamlSubjectID, "")
			if err != nil {
				return
			}
			logger.Info("Logout request: " + request.String())
			http.Redirect(w, req, request.String(), http.StatusFound)
		}
	})

	// /shib is accessed after authentication with the IDP. The post body contains the encrypted SAMLResponse.
	r.HandleFunc("POST /shib", func(w http.ResponseWriter, req *http.Request) {
		err := req.ParseForm()
		if err != nil {
			tools.WriteJSON(w, http.StatusBadRequest, map[string]any{"code": "400 - Bad Request", "error": err.Error()})
			return
		}
		response, err := getSamlSpFromHost(samlSPs, req.Host).ServiceProvider.ParseResponse(req, []string{""})
		if err != nil {
			tools.WriteJSON(w, http.StatusForbidden, map[string]any{"code": "403- Forbidden", "error": err.Error()})
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
				logger.Error("Can't extract mwn id", "LRZ-ID", lrzID, "firstName", firstName, "lastName", lastName, "mwnID", matrNr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			matrNr = s[0]
		}
		if firstName == "" {
			logger.Error("SAML response missing givenName attribute", "lrzID", lrzID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		user := model.User{
			Name:                firstName,
			LastName:            lastNameUser,
			MatriculationNumber: matrNr,
			LrzID:               lrzID,
		}
		err = daoWrapper.UsersDao.UpsertUser(&user)
		if err != nil {
			logger.Error("Could not upsert user", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		recordLogin(daoWrapper.UsersDao, user.ID)
		HandleValidLogin(w, req, &tools.SessionData{Userid: user.ID, SamlSubjectID: &subjectID})
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
