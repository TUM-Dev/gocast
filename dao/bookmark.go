package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=bookmark.go -destination ../mock_dao/bookmark.go

type BookmarkDao interface {
	Add(context.Context, *model.Bookmark) error
	GetByID(context.Context, uint) (model.Bookmark, error)
	GetByStreamID(context.Context, uint, uint) ([]model.Bookmark, error)
	Update(context.Context, *model.Bookmark) error
	Delete(context.Context, uint) error
}

type bookmarkDao struct {
	db *gorm.DB
}

func NewBookmarkDao() BookmarkDao {
	return bookmarkDao{db: DB}
}

func (d bookmarkDao) Add(ctx context.Context, bookmark *model.Bookmark) error {
	return d.db.WithContext(ctx).Save(bookmark).Error
}

func (d bookmarkDao) GetByID(ctx context.Context, id uint) (bookmark model.Bookmark, err error) {
	err = d.db.WithContext(ctx).Where("id = ?", id).First(&bookmark).Error
	return bookmark, err
}

func (d bookmarkDao) GetByStreamID(ctx context.Context, streamID uint, userID uint) (bookmarks []model.Bookmark, err error) {
	err = d.db.WithContext(ctx).Where("stream_id = ? AND user_id = ?", streamID, userID).Find(&bookmarks).Error
	return bookmarks, err
}

func (d bookmarkDao) Update(ctx context.Context, bookmark *model.Bookmark) error {
	return d.db.WithContext(ctx).Model(bookmark).Updates(bookmark).Error
}

func (d bookmarkDao) Delete(ctx context.Context, id uint) error {
	return d.db.Delete(&model.Bookmark{}, "id = ?", id).Error
}
