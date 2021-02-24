package dao

import (
	"TUM-Live-Backend/model"
	"context"
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
		Logger(ctx, "Create user.")
	}
	res := DB.Create(&session)
	return res.Error
}

func DeleteSession(ctx context.Context, session string) (err error) {
	if Logger != nil {
		Logger(ctx, "Create user.")
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

func GetUserBySID(ctx context.Context, sid string) (user model.User, err error) {
	if Logger != nil {
		Logger(ctx, "find user by email.")
	}
	var foundUser model.User
	dbErr := DB.Model(&model.User{}).
		Select("users.*").
		Joins("left join sessions on sessions.user_id = users.id").
		Where("session_id = ?", sid).
		Scan(&foundUser).Error
	return foundUser, dbErr
}
