package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=file.go -destination ../mock_dao/file.go

type FileDao interface {
	NewFile(f *model.File) error
	GetFileById(id string) (f model.File, err error)
	UpdateFile(id string, f *model.File) error
	DeleteFile(id uint) error
	CountVoDFiles() (int64, error)
	SetThumbnail(streamId uint, thumb model.File) error
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
	return
}

func (d fileDao) UpdateFile(id string, f *model.File) error {
	return DB.Model(&model.File{}).Where("id = ?", id).Updates(f).Error
}

func (d fileDao) DeleteFile(id uint) error {
	return DB.Model(&model.File{}).Delete(&model.File{}, id).Error
}

func (d fileDao) CountVoDFiles() (count int64, err error) {
	err = DB.Model(&model.File{}).Where("type = ?", model.FILETYPE_VOD).Count(&count).Error
	return
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
