package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=file.go -destination ../mock_dao/file.go

var File = NewFileDao()

type FileDao interface {
	NewFile(f *model.File) error
	GetFileById(id string) (f model.File, err error)
	UpdateFile(id string, f *model.File) error
	DeleteFile(id uint) error
}

type fileDao struct {
	db *gorm.DB
}

func NewFileDao() FileDao {
	return fileDao{db: DB}
}

func (d fileDao) NewFile(f *model.File) error {
	return DB.Create(f).Error
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
