package dao

import (
	"TUM-Live/model"
	"context"
	uuid "github.com/satori/go.uuid"
)

func AreUsersEmpty(ctx context.Context) (isEmpty bool, err error) {
	if Logger != nil {
		Logger(ctx, "Test if users table is empty.")
	}
	res := DB.Find(&model.User{})
	return res.RowsAffected == 0, res.Error
}

func CreateUser(ctx context.Context, user model.User) (err error) {
	if Logger != nil {
		Logger(ctx, "Create user.")
	}
	res := DB.Create(&user)
	return res.Error
}

func CreateSession(ctx context.Context, session model.Session) (err error) {
	if Logger != nil {
		Logger(ctx, "Create Session.")
	}
	res := DB.Create(&session)
	return res.Error
}

func DeleteSession(ctx context.Context, session string) (err error) {
	if Logger != nil {
		Logger(ctx, "Delete Session.")
	}
	res := DB.Delete(&model.Session{}, "session_id = ?", session)
	return res.Error
}

func GetUserByEmail(ctx context.Context, email string) (user model.User, err error) {
	if Logger != nil {
		Logger(ctx, "find user by email.")
	}
	var res model.User
	err = DB.First(&res, "email = ?", email).Error
	return res, err
}

func GetAllUsers(ctx context.Context, users *[]model.User) (err error) {
	if Logger != nil {
		Logger(ctx, "Get all users.")
	}
	err = DB.Find(users).Error
	return err
}

func GetUserBySID(ctx context.Context, sid string) (user model.User, err error) {
	if Logger != nil {
		Logger(ctx, "find user by session id "+sid)
	}
	var foundUser model.User
	dbErr := DB.Model(&model.User{}).
		Select("users.*").
		Joins("join sessions s on users.id = s.user_id").
		Where("session_key = ?", sid).
		Scan(&foundUser).Error
	return foundUser, dbErr
}

func CreateRegisterLink(ctx context.Context, user model.User) (registerLink model.RegisterLink, err error) {
	if Logger != nil {
		Logger(ctx, "generating a password reset link")
	}
	var link = uuid.NewV4().String()
	var registerLinkObj = model.RegisterLink{
		UserID:         user.ID,
		RegisterSecret: link,
	}
	err = DB.Create(&registerLinkObj).Error
	return registerLinkObj, err
}
