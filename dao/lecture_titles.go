package dao

import (
	"fmt"
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

	ExecAllCustomLectureTitlesBatched(f func([]StreamWithCustomLectureTitle, uint))
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

type StreamWithCustomLectureTitle struct {
	StreamID, UserID, CourseID uint
	Title, TeachingTerm        string
	Year                       int
}

// ExecAllCustomLectureTitlesBatched executes f on all streams (batched) with their courses and subtitles preloaded.
func (d userDefinedLectureTitlesDao) ExecAllCustomLectureTitlesBatched(f func([]StreamWithCustomLectureTitle, uint)) {
	var res []StreamWithCustomLectureTitle
	batchNum := uint(0)
	batchSize := uint(1000)
	var numLectureTitles int64
	DB.Model(&model.UserDefinedLectureTitle{}).Count(&numLectureTitles)
	var lastSeenStreamId, lastSeenUserId uint
	for int(batchSize)*int(batchNum) < int(numLectureTitles) {
		err := DB.Raw(`
				SELECT s.user_id, 
				       s.stream_id, 
				       s.title,
				       streams.course_id,
				       c.year, 
				       c.teaching_term
             	FROM user_defined_lecture_titles s 
             		  JOIN streams ON streams.id = s.stream_id
                      JOIN courses c ON c.id = streams.course_id
                WHERE s.user_id > ? AND s.stream_id > ?
				ORDER BY s.user_id, s.stream_id ASC
				LIMIT ? `, lastSeenUserId, lastSeenStreamId, batchSize).Scan(&res).Error
		if err != nil {
			fmt.Println(err)
		}
		if err == nil && len(res) > 0 {
			lastSeenUserId = res[len(res)-1].UserID
			lastSeenStreamId = res[len(res)-1].StreamID
		}
		f(res, batchSize*batchNum+1)
		batchNum++
	}
}
