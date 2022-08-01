package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=upload_key.go -destination ../mock_dao/upload_key.go

type UploadKeyDao interface {
	GetUploadKey(ctx context.Context, key string) (model.UploadKey, error)
	CreateUploadKey(ctx context.Context, key string, stream uint) error
	DeleteUploadKey(ctx context.Context, key model.UploadKey) error
}

type uploadKeyDao struct {
	db *gorm.DB
}

func (u uploadKeyDao) GetUploadKey(ctx context.Context, key string) (k model.UploadKey, err error) {
	return k, u.db.WithContext(ctx).Preload("Stream").First(&k, "upload_key = ?", key).Error
}

func (u uploadKeyDao) CreateUploadKey(ctx context.Context, key string, stream uint) error {
	return u.db.WithContext(ctx).Create(&model.UploadKey{UploadKey: key, StreamID: stream}).Error
}

func (u uploadKeyDao) DeleteUploadKey(ctx context.Context, key model.UploadKey) error {
	return u.db.WithContext(ctx).Unscoped().Delete(&key).Error
}

func NewUploadKeyDao() UploadKeyDao {
	return &uploadKeyDao{db: DB}
}
