package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=file.go -destination ../mock_dao/file.go

var File = NewFileDao()

type FileDao interface {
	GetFileById(id string) (f model.File, err error)
}

type fileDao struct {
	db *gorm.DB
}

func NewFileDao() FileDao {
	return fileDao{db: DB}
}

func (d fileDao) GetFileById(id string) (f model.File, err error) {
	err = DB.Where("id = ?", id).First(&f).Error
	return
}
