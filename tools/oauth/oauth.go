package oauth

import (
	"context"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var Auth *OAuth = &OAuth{}

type OAuth struct {
	LogoutURL    string
	ProviderURL  string
	Provider     *oidc.Provider        `default:"nil"`
	Verifier     *oidc.IDTokenVerifier `default:"nil"`
	OAuth2Config oauth2.Config         `default:"nil"`
	KeySet       *oidc.RemoteKeySet    `default:"nil"`
}

func (oauth *OAuth) SetupOauth() {
	if oauth.ProviderURL == "" && (tools.Cfg.OAuth == nil || tools.Cfg.OAuth.ProviderURL == "") {
		logger.Info("Provider URL is empty, oauth not enabled")
		return
	} else if oauth.ProviderURL == "" {
		oauth.ProviderURL = tools.Cfg.OAuth.ProviderURL
	}
	ctx := context.Background()
	var err error
	oauth.Provider, err = oidc.NewProvider(ctx, oauth.ProviderURL)
	if err != nil {
		logger.Error("Error creating provider for oauth", "err", err)
	}

	oauth.OAuth2Config = oauth2.Config{
		ClientID:     tools.Cfg.OAuth.ClientID,
		ClientSecret: tools.Cfg.OAuth.ClientSecret,
		RedirectURL:  tools.Cfg.OAuth.RedirectURL,

		Endpoint: oauth.Provider.Endpoint(),

		Scopes: []string{oidc.ScopeOpenID, "profile", "email", "roles"},
	}

	oauth.Verifier = oauth.Provider.Verifier(&oidc.Config{ClientID: oauth.OAuth2Config.ClientID})

	oauth.KeySet = oidc.NewRemoteKeySet(ctx, oauth.ProviderURL+"/protocol/openid-connect/certs")

	logger.Info("Successfully created OAuthConfig")
}
