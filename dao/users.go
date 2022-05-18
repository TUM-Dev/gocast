package dao

import (
	"context"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/model"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

//go:generate mockgen -source=users.go -destination ../mock_dao/users.go

type UsersDao interface {
	AreUsersEmpty(ctx context.Context) (isEmpty bool, err error)
	CreateUser(ctx context.Context, user *model.User) (err error)
	DeleteUser(ctx context.Context, uid uint) (err error)
	SearchUser(query string) (users []model.User, err error)
	IsUserAdmin(ctx context.Context, uid uint) (res bool, err error)
	GetUserByEmail(ctx context.Context, email string) (user model.User, err error)
	GetAllAdminsAndLecturers(users *[]model.User) (err error)
	GetUserByID(ctx context.Context, id uint) (user model.User, err error)
	CreateRegisterLink(ctx context.Context, user model.User) (registerLink model.RegisterLink, err error)
	GetUserByResetKey(key string) (model.User, error)
	DeleteResetKey(key string)
	UpdateUser(user model.User) error
	UpsertUser(user *model.User) error
	AddUsersToCourseByTUMIDs(matrNr []string, courseID uint) error
	AddUserSetting(userSetting *model.UserSetting) error
}

type usersDao struct {
	db *gorm.DB
}

func NewUsersDao() UsersDao {
	return usersDao{db: DB}
}

func (d usersDao) AreUsersEmpty(ctx context.Context) (isEmpty bool, err error) {
	_, found := Cache.Get("areUsersEmpty")
	if found {
		return false, nil
	}
	res := DB.Find(&model.User{})
	if res.RowsAffected != 0 {
		Cache.Set("areUsersEmpty", false, 1)
	}
	return res.RowsAffected == 0, res.Error
}

func (d usersDao) CreateUser(ctx context.Context, user *model.User) (err error) {
	res := DB.Create(&user)
	return res.Error
}

func (d usersDao) DeleteUser(ctx context.Context, uid uint) (err error) {
	res := DB.Delete(&model.User{}, "id = ?", uid)
	return res.Error
}

func (d usersDao) SearchUser(query string) (users []model.User, err error) {
	q := "%" + query + "%"
	res := DB.Where("UPPER(lrz_id) LIKE UPPER(?) OR UPPER(email) LIKE UPPER(?) OR UPPER(name) LIKE UPPER(?)", q, q, q).Limit(10).Find(&users)
	return users, res.Error
}

func (d usersDao) IsUserAdmin(ctx context.Context, uid uint) (res bool, err error) {
	var user model.User
	err = DB.Find(&user, "id = ?", uid).Error
	if err != nil {
		return false, err
	}
	return user.Role == 1, nil
}

func (d usersDao) GetUserByEmail(ctx context.Context, email string) (user model.User, err error) {
	var res model.User
	err = DB.First(&res, "email = ?", email).Error
	return res, err
}

func (d usersDao) GetAllAdminsAndLecturers(users *[]model.User) (err error) {
	err = DB.Find(users, "role < 3").Error
	return err
}

func (d usersDao) GetUserByID(ctx context.Context, id uint) (user model.User, err error) {
	if cached, found := Cache.Get(fmt.Sprintf("userById%d", id)); found {
		return cached.(model.User), nil
	}
	var foundUser model.User
	dbErr := DB.Preload("AdministeredCourses").Preload("Courses.Streams").Preload("Settings").Find(&foundUser, "id = ?", id).Error
	if dbErr == nil {
		Cache.SetWithTTL(fmt.Sprintf("userById%d", id), foundUser, 1, time.Second*10)
	}
	return foundUser, dbErr
}

func (d usersDao) CreateRegisterLink(ctx context.Context, user model.User) (registerLink model.RegisterLink, err error) {
	var link = uuid.NewV4().String()
	var registerLinkObj = model.RegisterLink{
		UserID:         user.ID,
		RegisterSecret: link,
	}
	err = DB.Create(&registerLinkObj).Error
	return registerLinkObj, err
}

func (d usersDao) GetUserByResetKey(key string) (model.User, error) {
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

func (d usersDao) DeleteResetKey(key string) {
	DB.Where("register_secret = ?", key).Delete(&model.RegisterLink{})
}

func (d usersDao) UpdateUser(user model.User) error {
	return DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&user).Error
}

func (d usersDao) UpsertUser(user *model.User) error {
	var foundUser *model.User
	err := DB.Model(&model.User{}).Where("matriculation_number = ?", user.MatriculationNumber).First(&foundUser).Error
	if err == nil && foundUser != nil {
		//User found: update
		user.Model = foundUser.Model
		foundUser.LrzID = user.LrzID
		foundUser.Name = user.Name
		if user.Role != 0 {
			foundUser.Role = user.Role
		}
		err := DB.Save(foundUser).Error
		if err != nil {
			return err
		}
		return nil
	}
	// user not found, create:
	user.Role = model.StudentType
	err = DB.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "matriculation_number"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"name": user.Name}),
		}).
		Create(&user).Error
	return err
}

func (d usersDao) AddUsersToCourseByTUMIDs(matrNr []string, courseID uint) error {
	// create empty users for ids that are not yet registered:
	stubUsers := make([]model.User, len(matrNr))
	for i, id := range matrNr {
		stubUsers[i] = model.User{MatriculationNumber: id, Role: model.StudentType}
	}
	DB.Model(&model.User{}).Clauses(clause.OnConflict{DoNothing: true}).Create(&stubUsers)

	// find users for current course:
	var foundUsersIDs []courseUsers
	err := DB.Model(&model.User{}).Where("matriculation_number in ?", matrNr).Select("? as course_id, id as user_id", courseID).Scan(&foundUsersIDs).Error
	if err != nil {
		return err
	}
	// add users to course
	err = DB.Table("course_users").Clauses(clause.OnConflict{DoNothing: true}).Create(&foundUsersIDs).Error
	if err != nil {
		return err
	}
	return nil
}

type courseUsers struct {
	CourseID uint
	UserID   uint
}

func (d usersDao) AddUserSetting(userSetting *model.UserSetting) error {
	defer Cache.Clear()
	err := d.db.Exec("DELETE FROM user_settings WHERE user_id = ? AND type = ?", userSetting.UserID, userSetting.Type).Error
	if err != nil {
		return err
	}
	return d.db.Create(userSetting).Error
}
