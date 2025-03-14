package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=lecture_titles.go -destination ../mock_dao/lecture_titles.go

type UserDefinedLectureTitlesDao interface {
	// Get UserDefinedLectureTitle by ID
	Get(uint, uint) (model.UserDefinedLectureTitle, error)

	// Delete a UserDefinedLectureTitle by user and stream id.
	Delete(uint, uint) error

	// Save updates the entry if it exists, inserts it else
	Save(userLectureTitle *model.UserDefinedLectureTitle) error
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

// Delete a userDefinedLectureTitlesDao by id.
func (d userDefinedLectureTitlesDao) Delete(userID uint, streamID uint) error {
	return d.db.Delete(&model.UserDefinedLectureTitle{}, "user_id = ? AND stream_id = ?", userID, streamID).Error
}

// Save updates the entry if it exists, inserts it else
func (d userDefinedLectureTitlesDao) Save(userLectureTitle *model.UserDefinedLectureTitle) error {
	return d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "stream_id"}}, // key column,
		DoUpdates: clause.AssignmentColumns([]string{"title"}),             // column needed to be updated
	}).Create(userLectureTitle).Error
}
