package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
)

func AreUsersEmpty(ctx context.Context) (isEmpty bool, err error) {
	_, found := Cache.Get("areUsersEmpty")
	if found {
		return false, nil
	}
	if Logger != nil {
		Logger(ctx, "Test if users table is empty.")
	}
	res := DB.Find(&model.User{})
	if res.RowsAffected != 0 {
		Cache.Set("areUsersEmpty", false, 1)
	}
	return res.RowsAffected == 0, res.Error
}

func CreateUser(ctx context.Context, user model.User) (err error) {
	if Logger != nil {
		Logger(ctx, "Create user.")
	}
	res := DB.Create(&user)
	return res.Error
}

func DeleteUser(ctx context.Context, uid uint) (err error) {
	if Logger != nil {
		Logger(ctx, "Delete User.")
	}
	res := DB.Unscoped().Delete(&model.User{}, "id = ?", uid)
	return res.Error
}

func IsUserAdmin(ctx context.Context, uid uint) (res bool, err error) {
	if Logger != nil {
		Logger(ctx, "Check if user is admin.")
	}
	var user model.User
	err = DB.Find(&user, "id = ?", uid).Error
	if err != nil {
		return false, err
	}
	return user.Role == 1, nil
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

func GetStudent(ctx context.Context, id string) (s model.Student, err error) {
	if Logger != nil {
		Logger(ctx, "find student by id: "+id)
	}
	var student model.Student
	dbErr := DB.Preload("Courses.Streams").Find(&student, "id = ?", id).Error
	return student, dbErr
}

func GetUserByID(ctx context.Context, id uint) (user model.User, err error) {
	if Logger != nil {
		Logger(ctx, fmt.Sprintf("find user by id %v", id))
	}
	var foundUser model.User
	dbErr := DB.Preload("Courses.Streams").Preload("InvitedCourses.Streams").Find(&foundUser, "id = ?", id).Error
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

func GetUserByResetKey(key string) (model.User, error) {
	var resetKey model.RegisterLink
	if err := DB.First(&resetKey, "register_secret = ?", key).Error; err != nil {
		return model.User{}, err
	}
	var user model.User
	if err := DB.First(&user, resetKey.UserID).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

func DeleteResetKey(key string) {
	DB.Where("register_secret = ?", key).Delete(&model.RegisterLink{})
}

func UpdateUser(user model.User) {
	if err := DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&user).Error; err != nil {
		log.Printf("error saving user: %v\n", err)
	}
}

func UpdateStudent(student model.Student) {
	// insert student if they don't exist yet.
	DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&student)
}
