package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=user_defined_lecture_titles.go -destination ../mock_dao/user_defined_lecture_titles.go

type UserDefinedLectureTitlesDao interface {
	// Get UserDefinedLectureTitle by ID
	Get(uint, uint) (model.UserDefinedLectureTitle, error)

	// Create a new UserDefinedLectureTitle for the database
	Create(*model.UserDefinedLectureTitle) error

	// Delete a UserDefinedLectureTitle by id.
	Delete(uint, uint) error

	// Upsert updates the entry if it exists, inserts it else
	Upsert(userLectureTitle *model.UserDefinedLectureTitle) error
}

type userDefinedLectureTitlesDao struct {
	db *gorm.DB
}

func NewUserDefinedLectureTitlesDao() UserDefinedLectureTitlesDao {
	return userDefinedLectureTitlesDao{db: DB}
}

// Get a userDefinedLectureTitlesDao by userID and streamID
func (d userDefinedLectureTitlesDao) Get(userID uint, streamID uint) (res model.UserDefinedLectureTitle, err error) {
	return res, d.db.First(&res, "user_id = ? AND stream_id = ?", userID, streamID).Error
}

// Create a userDefinedLectureTitlesDao.
func (d userDefinedLectureTitlesDao) Create(it *model.UserDefinedLectureTitle) error {
	return d.db.Create(it).Error
}

// Delete a userDefinedLectureTitlesDao by id.
func (d userDefinedLectureTitlesDao) Delete(userID uint, streamID uint) error {
	return d.db.Delete(&model.UserDefinedLectureTitle{}, "user_id = ? AND stream_id = ?", userID, streamID).Error
}

// Upsert updates the entry if it exists, inserts it else
func (d userDefinedLectureTitlesDao) Upsert(userLectureTitle *model.UserDefinedLectureTitle) error {
	return d.db.Save(userLectureTitle).Error
}
