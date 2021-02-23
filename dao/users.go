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


func CreateUser(ctx context.Context, user model.User) (err error){
	if Logger != nil {
		Logger(ctx, "Create user.")
	}
	res := DB.Create(&user)
	return res.Error
}

func CreateSession(ctx context.Context, session model.Session) (err error){
	if Logger != nil {
		Logger(ctx, "Create user.")
	}
	res := DB.Create(&session)
	return res.Error
}

func GetUserByEmail(ctx context.Context, email string) (user model.User, err error)  {
	if Logger != nil {
		Logger(ctx, "find user by email.")
	}
	var res model.User
	err = DB.First(&res, "email = ?", email).Error
	return res, err
}
