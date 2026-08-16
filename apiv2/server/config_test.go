package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/tools"
)

// The first thing the frontend asks for, before anyone has signed in, so it has to
// work on a deployment that has configured nothing and has no users.
func TestGetFrontendConfig(t *testing.T) {
	tools.BrandingCfg = tools.Branding{Title: "GoCast", Description: "lecture streaming"}
	tools.VersionTag = "v1.2.3"
	tools.Cfg.WikiURL = "https://wiki.example"
	tools.Cfg.CanonicalURL = "https://tum.live"

	t.Run("reports the deployment's configuration", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		usersMock := mock_dao.NewMockUsersDao(ctrl)
		usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil).Times(1)

		api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

		resp, err := api.GetFrontendConfig(context.Background(), &emptypb.Empty{})
		if err != nil {
			t.Fatalf("GetFrontendConfig: %v", err)
		}

		if resp.Branding.GetTitle() != "GoCast" || resp.Branding.GetDescription() != "lecture streaming" {
			t.Errorf("branding = %+v, want the configured one", resp.Branding)
		}
		if resp.VersionTag != "v1.2.3" {
			t.Errorf("versionTag = %q, want v1.2.3", resp.VersionTag)
		}
		if resp.WikiUrl != "https://wiki.example" || resp.CanonicalUrl != "https://tum.live" {
			t.Errorf("links = %q / %q, want the configured ones", resp.WikiUrl, resp.CanonicalUrl)
		}
		if resp.IsFreshInstallation {
			t.Error("reported a fresh installation for a database that has users")
		}
	})

	t.Run("reports a database with no users as a fresh installation", func(t *testing.T) {
		// Getting this wrong shows a login form nobody can use.
		ctrl := gomock.NewController(t)
		usersMock := mock_dao.NewMockUsersDao(ctrl)
		usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(true, nil).Times(1)

		api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

		resp, err := api.GetFrontendConfig(context.Background(), &emptypb.Empty{})
		if err != nil {
			t.Fatalf("GetFrontendConfig: %v", err)
		}
		if !resp.IsFreshInstallation {
			t.Error("did not report a fresh installation for an empty database")
		}
	})

	t.Run("a database failure is a 500, not a silent fresh installation", func(t *testing.T) {
		// Defaulting to false would show a login form nobody can use, unexplained.
		ctrl := gomock.NewController(t)
		usersMock := mock_dao.NewMockUsersDao(ctrl)
		usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, errors.New("database is down")).Times(1)

		api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

		_, err := api.GetFrontendConfig(context.Background(), &emptypb.Empty{})
		if got := status.Code(err); got != codes.Unknown {
			t.Errorf("code = %v, want an error status", got)
		}
	})
}
