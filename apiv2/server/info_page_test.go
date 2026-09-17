package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestGetInfoPage(t *testing.T) {
	t.Run("renders a page found by slug", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetBySlug("about").
			Return(model.InfoPage{Name: "About", RawContent: "# About", Type: model.INFOPAGE_MARKDOWN}, nil).
			Times(1)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		resp, err := api.GetInfoPage(context.Background(), &protobuf.GetInfoPageRequest{Name: "about"})
		if err != nil {
			t.Fatalf("GetInfoPage: %v", err)
		}
		if resp.Name != "about" {
			t.Errorf("name = %q, want %q", resp.Name, "about")
		}
		if resp.Content == "" {
			t.Error("content is empty, want the rendered page")
		}
	})

	t.Run("404s a slug nothing was seeded for", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetBySlug("nope").Return(model.InfoPage{}, gorm.ErrRecordNotFound).Times(1)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		if _, err := api.GetInfoPage(context.Background(), &protobuf.GetInfoPageRequest{Name: "nope"}); err == nil {
			t.Fatal("an unknown slug was not reported as an error")
		}
	})
}

func TestListInfoPages(t *testing.T) {
	ctrl := gomock.NewController(t)
	infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
	infoPageMock.EXPECT().GetAll().Return([]model.InfoPage{
		{Slug: "privacy", Name: "Privacy Policy", RawContent: "secret content"},
	}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

	resp, err := api.ListInfoPages(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListInfoPages: %v", err)
	}
	if len(resp.Pages) != 1 || resp.Pages[0].Slug != "privacy" {
		t.Fatalf("got %+v, want one page with slug 'privacy'", resp.Pages)
	}
	// The summary is for visitors deciding whether to ask getInfoPage for the page;
	// it must not carry content of its own.
	if resp.Pages[0].Name != "Privacy Policy" {
		t.Errorf("name = %q, want %q", resp.Pages[0].Name, "Privacy Policy")
	}
}

func TestCreateInfoPage(t *testing.T) {
	t.Run("creates a page with a well-formed, free slug", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetBySlug("terms-of-use").Return(model.InfoPage{}, gorm.ErrRecordNotFound).Times(1)
		infoPageMock.EXPECT().New(gomock.Any()).DoAndReturn(func(page *model.InfoPage) error {
			page.ID = 7
			return nil
		}).Times(1)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		resp, err := api.CreateInfoPage(context.Background(), &protobuf.CreateInfoPageRequest{
			Slug: "terms-of-use", Name: "Terms of Use", RawContent: "# Terms",
		})
		if err != nil {
			t.Fatalf("CreateInfoPage: %v", err)
		}
		if resp.Id != 7 || resp.Slug != "terms-of-use" {
			t.Errorf("got %+v", resp)
		}
	})

	t.Run("rejects a slug that is not lowercase kebab-case", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetBySlug(gomock.Any()).Times(0)
		infoPageMock.EXPECT().New(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		_, err := api.CreateInfoPage(context.Background(), &protobuf.CreateInfoPageRequest{
			Slug: "Terms Of Use!", Name: "Terms of Use",
		})
		if err == nil {
			t.Fatal("a malformed slug was accepted")
		}
	})

	t.Run("rejects a slug another route already owns", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetBySlug(gomock.Any()).Times(0)
		infoPageMock.EXPECT().New(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		_, err := api.CreateInfoPage(context.Background(), &protobuf.CreateInfoPageRequest{
			Slug: "login", Name: "Not Actually Login",
		})
		if err == nil {
			t.Fatal("a reserved slug was accepted")
		}
	})

	t.Run("rejects a slug already taken by another page", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetBySlug("privacy").Return(model.InfoPage{Slug: "privacy"}, nil).Times(1)
		infoPageMock.EXPECT().New(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		_, err := api.CreateInfoPage(context.Background(), &protobuf.CreateInfoPageRequest{
			Slug: "privacy", Name: "Another Privacy Page",
		})
		if err == nil {
			t.Fatal("a duplicate slug was accepted")
		}
	})

	t.Run("rejects a blank title", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetBySlug(gomock.Any()).Times(0)
		infoPageMock.EXPECT().New(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		_, err := api.CreateInfoPage(context.Background(), &protobuf.CreateInfoPageRequest{
			Slug: "terms", Name: "   ",
		})
		if err == nil {
			t.Fatal("a blank title was accepted")
		}
	})
}

func TestUpdateInfoPage(t *testing.T) {
	t.Run("updates an existing page", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetById(uint(3)).Return(model.InfoPage{Model: gorm.Model{ID: 3}, Slug: "about"}, nil).Times(1)
		infoPageMock.EXPECT().GetBySlug("about").Return(model.InfoPage{Model: gorm.Model{ID: 3}, Slug: "about"}, nil).Times(1)
		infoPageMock.EXPECT().Update(uint(3), gomock.Any()).Return(nil).Times(1)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		resp, err := api.UpdateInfoPage(context.Background(), &protobuf.UpdateInfoPageRequest{
			Id: 3, Slug: "about", Name: "About Us", RawContent: "# About",
		})
		if err != nil {
			t.Fatalf("UpdateInfoPage: %v", err)
		}
		if resp.Id != 3 {
			t.Errorf("id = %d, want 3", resp.Id)
		}
	})

	t.Run("404s an id nothing was seeded for", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetById(uint(99)).Return(model.InfoPage{}, nil).Times(1)
		infoPageMock.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		_, err := api.UpdateInfoPage(context.Background(), &protobuf.UpdateInfoPageRequest{
			Id: 99, Slug: "about", Name: "About",
		})
		if err == nil {
			t.Fatal("a missing page was not reported as an error")
		}
	})

	t.Run("rejects renaming onto a slug another page already holds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().GetById(uint(3)).Return(model.InfoPage{Model: gorm.Model{ID: 3}, Slug: "about"}, nil).Times(1)
		infoPageMock.EXPECT().GetBySlug("privacy").Return(model.InfoPage{Model: gorm.Model{ID: 1}, Slug: "privacy"}, nil).Times(1)
		infoPageMock.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		_, err := api.UpdateInfoPage(context.Background(), &protobuf.UpdateInfoPageRequest{
			Id: 3, Slug: "privacy", Name: "About",
		})
		if err == nil {
			t.Fatal("renaming onto another page's slug was accepted")
		}
	})
}

func TestDeleteInfoPage(t *testing.T) {
	t.Run("deletes the page it was given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().Delete(uint(3)).Return(nil).Times(1)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		if _, err := api.DeleteInfoPage(context.Background(), &protobuf.DeleteInfoPageRequest{Id: 3}); err != nil {
			t.Fatalf("DeleteInfoPage: %v", err)
		}
	})

	t.Run("reports a failed delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoPageMock := mock_dao.NewMockInfoPageDao(ctrl)
		infoPageMock.EXPECT().Delete(gomock.Any()).Return(errors.New("database is on fire")).Times(1)

		api := &API{dao: dao.DaoWrapper{InfoPageDao: infoPageMock}, log: slog.Default()}

		_, err := api.DeleteInfoPage(context.Background(), &protobuf.DeleteInfoPageRequest{Id: 3})
		if err == nil {
			t.Fatal("a failed delete was reported as success")
		}
	})
}
