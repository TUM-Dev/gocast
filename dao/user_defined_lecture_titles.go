package dao

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=user_defined_lecture_titles.go -destination ../mock_dao/user_defined_lecture_titles.go

type UserDefinedLectureTitlesDao interface {
	// Get UserDefinedLectureTitle by ID
	Get(context.Context, uint, uint) (model.UserDefinedLectureTitle, error)

	// Create a new UserDefinedLectureTitle for the database
	Create(context.Context, *model.UserDefinedLectureTitle) error

	// Delete a UserDefinedLectureTitle by id.
	Delete(context.Context, uint, uint) error

	// UpdateOrCreate updates the entry if it exists, creates it else
	UpdateOrCreate(c context.Context, userLectureTitle *model.UserDefinedLectureTitle) error
}

type userDefinedLectureTitlesDao struct {
	db *gorm.DB
}

func NewUserDefinedLectureTitlesDao() UserDefinedLectureTitlesDao {
	return userDefinedLectureTitlesDao{db: DB}
}

// Get a userDefinedLectureTitlesDao by userID and streamID
func (d userDefinedLectureTitlesDao) Get(c context.Context, userID uint, streamID uint) (res model.UserDefinedLectureTitle, err error) {
	return res, d.db.WithContext(c).First(&res, "user_id = ? AND stream_id = ?", userID, streamID).Error
}

// Create a userDefinedLectureTitlesDao.
func (d userDefinedLectureTitlesDao) Create(c context.Context, it *model.UserDefinedLectureTitle) error {
	return d.db.WithContext(c).Create(it).Error
}

// Delete a userDefinedLectureTitlesDao by id.
func (d userDefinedLectureTitlesDao) Delete(c context.Context, userID uint, streamID uint) error {
	return d.db.WithContext(c).Delete(&model.UserDefinedLectureTitle{}, "user_id = ? AND stream_id = ?", userID, streamID).Error
}

// UpdateOrCreate updates the entry if it exists, creates it else
func (d userDefinedLectureTitlesDao) UpdateOrCreate(c context.Context, userLectureTitle *model.UserDefinedLectureTitle) error {
	return d.db.WithContext(c).Save(userLectureTitle).Error
}
