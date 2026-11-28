package dao

import (
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=media_reap.go -destination ../mock_dao/media_reap.go

// MediaReapDao reads what the media reaper needs: every stream and File row, soft-deleted
// ones included. Both reads select only the columns used, because the reaper holds the
// whole set in memory to tell which files a live stream still references.
type MediaReapDao interface {
	// ReapStreams returns every stream, soft-deleted or not.
	ReapStreams() ([]ReapStream, error)
	// ReapFiles returns every File row except the deprecated VOD type, whose paths are
	// legacy locations the reaper does not handle.
	ReapFiles() ([]ReapFile, error)
}

type ReapStream struct {
	ID                                           uint
	CourseID                                     uint
	DeletedAt                                    gorm.DeletedAt
	PlaylistUrl, PlaylistUrlPRES, PlaylistUrlCAM string
}

type ReapFile struct {
	StreamID  uint
	Path      string
	DeletedAt gorm.DeletedAt
}

func (r ReapStream) Deleted() *time.Time {
	if !r.DeletedAt.Valid {
		return nil
	}
	t := r.DeletedAt.Time
	return &t
}

type mediaReapDao struct {
	db *gorm.DB
}

func NewMediaReapDao() MediaReapDao {
	return mediaReapDao{db: DB}
}

func (d mediaReapDao) ReapStreams() ([]ReapStream, error) {
	var out []ReapStream
	err := DB.Unscoped().Model(&model.Stream{}).
		Select("id, course_id, deleted_at, playlist_url, playlist_url_pres, playlist_url_cam").
		Find(&out).Error
	return out, err
}

func (d mediaReapDao) ReapFiles() ([]ReapFile, error) {
	var out []ReapFile
	err := DB.Unscoped().Model(&model.File{}).
		Select("stream_id, path, deleted_at").
		Where("type <> ?", model.FILETYPE_VOD).
		Find(&out).Error
	return out, err
}
