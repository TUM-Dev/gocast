package dao

import (
	"math"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=lecture_cut.go -destination ../mock_dao/lecture_cut.go

type LectureCutDao interface {
	// SaveLectureCut records a cut that is under way, replacing the one a failed try left.
	SaveLectureCut(cut model.LectureCut) error
	// GetLectureCut is the cut under way that makes the VODs of the lecture.
	GetLectureCut(targetStreamID uint) (model.LectureCut, error)
	// DeleteLectureCut drops a cut that was not carried out.
	DeleteLectureCut(targetStreamID uint) error
	// FinishLectureCut moves what the lecture has that refers to times in the recording onto
	// the cut one, and ends the cut. All of it or none, so that a second try after a failure
	// cannot move it twice. Two versions finishing at once both get here: only the one that
	// takes the cut's row does the work, and finished says whether that was this call.
	//
	// The offsets and video sections are moved, the silences, which are measured on the
	// recording, are removed for a scan of the new one. A duplicate takes over the
	// visibility the source had and the source becomes private.
	FinishLectureCut(cut model.LectureCut) (finished bool, err error)
}

type lectureCutDao struct {
	db *gorm.DB
}

func NewLectureCutDao() LectureCutDao {
	return lectureCutDao{db: DB}
}

func (d lectureCutDao) SaveLectureCut(cut model.LectureCut) error {
	return DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&cut).Error
}

func (d lectureCutDao) GetLectureCut(targetStreamID uint) (cut model.LectureCut, err error) {
	err = DB.Where("target_stream_id = ?", targetStreamID).First(&cut).Error
	return cut, err
}

func (d lectureCutDao) DeleteLectureCut(targetStreamID uint) error {
	return DB.Where("target_stream_id = ?", targetStreamID).Delete(&model.LectureCut{}).Error
}

func (d lectureCutDao) FinishLectureCut(cut model.LectureCut) (finished bool, err error) {
	defer Cache.Clear()
	err = DB.Transaction(func(tx *gorm.DB) error {
		// First, so that a concurrent call blocks on the row until this one commits and then
		// finds it gone.
		claimed := tx.Delete(&model.LectureCut{}, "target_stream_id = ?", cut.TargetStreamID)
		if claimed.Error != nil || claimed.RowsAffected == 0 {
			return claimed.Error
		}
		finished = true

		var stream model.Stream
		if err := tx.First(&stream, cut.TargetStreamID).Error; err != nil {
			return err
		}

		updates := map[string]any{}
		// The offsets are milliseconds, the sections are not.
		if stream.StartOffset > 0 {
			updates["start_offset"] = remapMillis(stream.StartOffset, cut.Discard)
		}
		if stream.EndOffset > 0 {
			updates["end_offset"] = remapMillis(stream.EndOffset, cut.Discard)
		}
		if cut.Duplicate() {
			updates["private"] = cut.SourcePrivate
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.Stream{}).Where("id = ?", stream.ID).Updates(updates).Error; err != nil {
				return err
			}
		}

		var sections []model.VideoSection
		if err := tx.Where("stream_id = ?", stream.ID).Find(&sections).Error; err != nil {
			return err
		}
		for _, s := range sections {
			at := uint(math.Round(model.RemapTime(float64(s.StartHours*3600+s.StartMinutes*60+s.StartSeconds), cut.Discard)))
			err := tx.Model(&model.VideoSection{}).Where("id = ?", s.ID).Updates(map[string]any{
				"start_hours": at / 3600, "start_minutes": at % 3600 / 60, "start_seconds": at % 60,
			}).Error
			if err != nil {
				return err
			}
		}

		if err := tx.Delete(&model.Silence{}, "stream_id = ?", stream.ID).Error; err != nil {
			return err
		}
		if cut.Duplicate() {
			if err := tx.Model(&model.Stream{}).Where("id = ?", cut.SourceStreamID).Update("private", true).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		finished = false
	}
	return finished, err
}

func remapMillis(ms uint, discard []model.CutRange) uint {
	return uint(math.Round(model.RemapTime(float64(ms)/1000, discard) * 1000))
}
