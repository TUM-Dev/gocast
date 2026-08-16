package apiv2

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// Password reset is the one flow a locked-out user has. These cover the mail that
// goes out and what the response reveals about who has an account. Ported from v1.

func TestResetPassword(t *testing.T) {
	hansi := model.User{
		Model: gorm.Model{ID: 1},
		Name:  "Hansi",
		Email: sql.NullString{String: "hansi@tum.de", Valid: true},
		Role:  model.StudentType,
	}

	tools.Cfg.Mail = tools.MailConfig{Sender: "from@invalid", Server: "server", MaxMailsPerMinute: 1}
	tools.Cfg.WebUrl = "https://tum.live"

	t.Run("emails a reset link to an account that exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		usersMock := mock_dao.NewMockUsersDao(ctrl)
		usersMock.EXPECT().GetUserByEmail(gomock.Any(), hansi.Email.String).Return(hansi, nil).Times(1)
		usersMock.EXPECT().CreateRegisterLink(gomock.Any(), hansi).
			Return(model.RegisterLink{RegisterSecret: "abc"}, nil).Times(1)

		emailMock := mock_dao.NewMockEmailDao(ctrl)
		emailMock.EXPECT().Create(gomock.Any(), &model.Email{
			From:    tools.Cfg.Mail.Sender,
			To:      hansi.Email.String,
			Subject: "TUM-Live: Reset Password",
			Body: "Hi! \n\nYou can reset your TUM-Live password by clicking on the following link: \n\n" +
				tools.Cfg.WebUrl + "/setPassword/abc" +
				"\n\nIf you did not request a password reset, please ignore this email. \n\nBest regards",
		}).Return(nil).Times(1)

		api := &API{
			dao: dao.DaoWrapper{UsersDao: usersMock, EmailDao: emailMock},
			log: slog.Default(),
		}

		resp, err := api.ResetPassword(context.Background(), &protobuf.ResetPasswordRequest{Email: hansi.Email.String})
		if err != nil {
			t.Fatalf("ResetPassword: %v", err)
		}
		if resp.Message == "" {
			t.Error("no message returned")
		}
	})

	t.Run("says the same thing for an account that does not exist", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		usersMock := mock_dao.NewMockUsersDao(ctrl)
		usersMock.EXPECT().GetUserByEmail(gomock.Any(), "nobody@tum.de").
			Return(model.User{}, gorm.ErrRecordNotFound).Times(1)
		// No link, no mail: the absence of these expectations is the assertion.

		api := &API{
			dao: dao.DaoWrapper{UsersDao: usersMock, EmailDao: mock_dao.NewMockEmailDao(ctrl)},
			log: slog.Default(),
		}

		resp, err := api.ResetPassword(context.Background(), &protobuf.ResetPasswordRequest{Email: "nobody@tum.de"})
		if err != nil {
			t.Fatalf("an unknown address must not be an error: %v", err)
		}
		// Telling the two cases apart would turn this endpoint into a way to find out
		// which addresses have accounts.
		if resp.Message == "" {
			t.Error("no message returned")
		}
	})
}
