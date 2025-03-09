// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// GetUser retrieves the current user based on the context.
// It returns a GetUserResponse or an error if one occurs.
func (a *API) GetUser(ctx context.Context, req *emptypb.Empty) (*protobuf.GetUserResponse, error) {
	a.log.Info("GetUser")

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	resp := &protobuf.GetUserResponse{User: h.ParseUserToProto(user)}

	return resp, nil
}

// UpdateUserSettings updates the profile settings for the current user.
func (a *API) UpdateUserSettings(ctx context.Context, req *protobuf.UpdateUserSettingsRequest) (*protobuf.UpdateUserSettingsResponse, error) {
	a.log.Info("UpdateUserSettings")

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	settings, err := h.UpdateUserSettings(a.db, user, req)
	if err != nil {
		return nil, err
	}

	resp := make([]*protobuf.UserSetting, len(settings))

	for i, setting := range settings {
		resp[i] = h.ParseUserSettingToProto(setting)
	}

	return &protobuf.UpdateUserSettingsResponse{UserSettings: resp}, nil
}

// ResetPassword resets the password for the user with the given username.
func (a *API) ResetPassword(ctx context.Context, req *protobuf.ResetPasswordRequest) (*protobuf.ResetPasswordResponse, error) {
	a.log.Info("ResetPassword")

	user, err := a.dao.UsersDao.GetUserByEmail(ctx, req.Email)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// wrong username/email -> pass
		return &protobuf.ResetPasswordResponse{Message: "If the email exists, a reset link has been sent."}, nil
	}
	if err != nil {
		a.log.Error("can't get user for password reset", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	link, err := a.dao.UsersDao.CreateRegisterLink(ctx, user)
	if err != nil {
		a.log.Error("can't create register link", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	err = a.dao.EmailDao.Create(ctx, &model.Email{
		From:    tools.Cfg.Mail.Sender,
		To:      user.Email.String,
		Subject: "TUM-Live: Reset Password",
		Body:    "Hi! \n\nYou can reset your TUM-Live password by clicking on the following link: \n\n" + tools.Cfg.WebUrl + "/setPassword/" + link.RegisterSecret + "\n\nIf you did not request a password reset, please ignore this email. \n\nBest regards",
	})
	if err != nil {
		a.log.Error("can't save reset password email", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.ResetPasswordResponse{Message: "If the username exists, a reset link has been sent."}, nil
}

// ExportPersonalData exports the personal data of the current user.
func (a *API) ExportPersonalData(ctx context.Context, req *emptypb.Empty) (*protobuf.ExportPersonalDataResponse, error) {
	a.log.Info("ExportPersonalData")

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	resp := &protobuf.ExportPersonalDataResponse{UserData: h.ParseUserToProto(user)}

	for _, course := range user.Courses {
		resp.Enrollments = append(resp.Enrollments, &protobuf.Enrollment{
			Year:   int32(course.Year),
			Term:   course.TeachingTerm,
			Course: course.Name,
		})
	}

	progresses, err := a.dao.ProgressDao.GetProgressesForUser(user.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	for _, progress := range progresses {
		resp.VideoViews = append(resp.VideoViews, &protobuf.VideoView{
			StreamId:       uint32(progress.StreamID),
			Progress:       float32(progress.Progress),
			MarkedFinished: progress.Watched,
		})
	}

	chats, err := a.dao.ChatDao.GetChatsByUser(user.ID)
	if err != nil {
		chats = []model.Chat{}
	}

	for _, chat := range chats {
		resp.Chats = append(resp.Chats, &protobuf.Chat{
			StreamId:  uint32(chat.StreamID),
			Message:   chat.Message,
			CreatedAt: timestamppb.New(chat.CreatedAt),
		})
	}

	return resp, nil
}
