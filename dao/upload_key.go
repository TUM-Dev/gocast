package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

type UploadKeyDao interface {
	GetUploadKey(key string) (model.UploadKey, error)
	CreateUploadKey(key string, stream uint) error
	DeleteUploadKey(key model.UploadKey) error
}

type uploadKeyDao struct {
	db *gorm.DB
}

func (u uploadKeyDao) GetUploadKey(key string) (k model.UploadKey, err error) {
	return k, u.db.Preload("Stream").First(&k, "upload_key = ?", key).Error
}

func (u uploadKeyDao) CreateUploadKey(key string, stream uint) error {
	return u.db.Create(&model.UploadKey{UploadKey: key, StreamID: stream}).Error
}

func (u uploadKeyDao) DeleteUploadKey(key model.UploadKey) error {
	return u.db.Unscoped().Delete(&key).Error
}

func NewUploadKeyDao() UploadKeyDao {
	return &uploadKeyDao{db: DB}
}
