package dao

import (
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=file.go -destination ../mock_dao/file.go

type FileDao interface {
	NewFile(f *model.File) error
	GetFileById(id string) (f model.File, err error)
	DeleteFile(id uint) error
	CountVoDFiles() (int64, error)
	SetThumbnail(streamId uint, thumb model.File) error
	GetThumbnail(streamId uint, fileType model.FileType) (f model.File, err error)
	// SetStreamFile makes file the only file of its type the stream has.
	SetStreamFile(streamId uint, file model.File) error
	// GetStreamFile is the stream's file of the type, gorm.ErrRecordNotFound if it has none.
	GetStreamFile(streamId uint, fileType model.FileType) (f model.File, err error)
	// DeleteStreamFiles removes the rows of the stream's files of the type, not the files.
	DeleteStreamFiles(streamId uint, fileType model.FileType) error
}

type fileDao struct {
	db *gorm.DB
}

func NewFileDao() FileDao {
	return fileDao{db: DB}
}

func (d fileDao) NewFile(f *model.File) error {
	return DB.Create(&f).Error
}

func (d fileDao) GetFileById(id string) (f model.File, err error) {
	err = DB.Where("id = ?", id).First(&f).Error
	return f, err
}

func (d fileDao) DeleteFile(id uint) error {
	return DB.Model(&model.File{}).Delete(&model.File{}, id).Error
}

func (d fileDao) CountVoDFiles() (count int64, err error) {
	err = DB.Model(&model.File{}).Where("type = ?", model.FILETYPE_VOD).Count(&count).Error
	return count, err
}

func (d fileDao) SetThumbnail(streamId uint, thumb model.File) error {
	defer Cache.Clear()
	return DB.Transaction(func(tx *gorm.DB) error {
		err := DB.Where("stream_id = ? AND type = ?", streamId, thumb.Type).Delete(&model.File{}).Error
		if err != nil {
			return err
		}
		return tx.Create(&thumb).Error
	})
}

func (d fileDao) GetThumbnail(streamId uint, fileType model.FileType) (f model.File, err error) {
	err = DB.Where("stream_id = ? AND type = ?", streamId, fileType).First(&f).Error
	return f, err
}

func (d fileDao) SetStreamFile(streamId uint, file model.File) error {
	defer Cache.Clear()
	file.StreamID = streamId
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("stream_id = ? AND type = ?", streamId, file.Type).Delete(&model.File{}).Error; err != nil {
			return err
		}
		return tx.Create(&file).Error
	})
}

func (d fileDao) GetStreamFile(streamId uint, fileType model.FileType) (f model.File, err error) {
	err = DB.Where("stream_id = ? AND type = ?", streamId, fileType).First(&f).Error
	return f, err
}

func (d fileDao) DeleteStreamFiles(streamId uint, fileType model.FileType) error {
	defer Cache.Clear()
	return DB.Where("stream_id = ? AND type = ?", streamId, fileType).Delete(&model.File{}).Error
}
