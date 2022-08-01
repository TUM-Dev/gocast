package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=file.go -destination ../mock_dao/file.go

type FileDao interface {
	NewFile(ctx context.Context, f *model.File) error
	GetFileById(ctx context.Context, id string) (f model.File, err error)
	UpdateFile(ctx context.Context, id string, f *model.File) error
	DeleteFile(ctx context.Context, id uint) error
}

type fileDao struct {
	db *gorm.DB
}

func NewFileDao() FileDao {
	return fileDao{db: DB}
}

func (d fileDao) NewFile(ctx context.Context, f *model.File) error {
	return DB.WithContext(ctx).Create(&f).Error
}

func (d fileDao) GetFileById(ctx context.Context, id string) (f model.File, err error) {
	err = DB.WithContext(ctx).Where("id = ?", id).First(&f).Error
	return
}

func (d fileDao) UpdateFile(ctx context.Context, id string, f *model.File) error {
	return DB.WithContext(ctx).Model(&model.File{}).Where("id = ?", id).Updates(f).Error
}

func (d fileDao) DeleteFile(ctx context.Context, id uint) error {
	return DB.WithContext(ctx).Model(&model.File{}).Delete(&model.File{}, id).Error
}
