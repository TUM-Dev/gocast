package apiv2

import (
	"context"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/tools"
)

// GetFrontendConfig returns what the frontend needs to render its shell: branding,
// version and footer links. Public, because the shell renders before anyone signs
// in. Nothing in it is per-user.
func (a *API) GetFrontendConfig(ctx context.Context, req *emptypb.Empty) (*protobuf.GetFrontendConfigResponse, error) {
	// No users means nobody can sign in yet, so the frontend offers to create the
	// first account instead of a login form.
	fresh, err := a.dao.UsersDao.AreUsersEmpty(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.GetFrontendConfigResponse{
		Branding: &protobuf.Branding{
			Title:       tools.BrandingCfg.Title,
			Description: tools.BrandingCfg.Description,
		},
		VersionTag:          tools.VersionTag,
		WikiUrl:             tools.Cfg.WikiURL,
		CanonicalUrl:        tools.Cfg.CanonicalURL,
		IsFreshInstallation: fresh,
	}, nil
}
