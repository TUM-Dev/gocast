package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=bookmark.go -destination ../mock_dao/bookmark.go

type BookmarkDao interface {
	Add(*model.Bookmark) error
	GetByStreamID(uint) ([]model.Bookmark, error)
	Update(*model.Bookmark) error
	Delete(uint) error
}

type bookmarkDao struct {
	db *gorm.DB
}

func NewBookmarkDao() BookmarkDao {
	return bookmarkDao{db: DB}
}

func (d bookmarkDao) Add(bookmark *model.Bookmark) error {
	return d.db.Save(bookmark).Error
}

func (d bookmarkDao) GetByStreamID(streamID uint) (bookmarks []model.Bookmark, err error) {
	err = d.db.Find(bookmarks, "stream_id = ?", streamID).Error
	return bookmarks, err
}

func (d bookmarkDao) Update(bookmark *model.Bookmark) error {
	return d.db.Model(bookmark).Updates(bookmark).Error
}

func (d bookmarkDao) Delete(id uint) error {
	return d.db.Delete(&model.Bookmark{}, "id = ?", id).Error
}
