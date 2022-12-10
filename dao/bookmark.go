package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=bookmark.go -destination ../mock_dao/bookmark.go

type BookmarkDao interface {
	Add(*model.Bookmark) error
	GetByID(uint) (model.Bookmark, error)
	GetByStreamID(uint, uint) ([]model.Bookmark, error)
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

func (d bookmarkDao) GetByID(id uint) (bookmark model.Bookmark, err error) {
	err = d.db.Where("id = ?", id).First(&bookmark).Error
	return bookmark, err
}

func (d bookmarkDao) GetByStreamID(streamID uint, userID uint) (bookmarks []model.Bookmark, err error) {
	err = d.db.Order("hours, minutes, seconds ASC").Where("stream_id = ? AND user_id = ?", streamID, userID).Find(&bookmarks).Error
	return bookmarks, err
}

func (d bookmarkDao) Update(bookmark *model.Bookmark) error {
	return d.db.Model(bookmark).Updates(bookmark).Error
}

func (d bookmarkDao) Delete(id uint) error {
	return d.db.Delete(&model.Bookmark{}, "id = ?", id).Error
}
