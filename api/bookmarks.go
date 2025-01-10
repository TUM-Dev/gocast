package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func configGinBookmarksRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := bookmarkRoutes{daoWrapper}
	bookmarks := router.Group("/api/bookmarks")
	{
		bookmarks.Use(tools.LoggedIn)
		bookmarks.POST("", routes.Add)
		bookmarks.GET("", routes.GetByStreamID)
		bookmarks.PUT("/:id", routes.Update)
		bookmarks.DELETE("/:id", routes.Delete)
	}
}

type bookmarkRoutes struct {
	dao.DaoWrapper
}

type AddBookmarkRequest struct {
	StreamID    uint   `json:"streamID"`
	Description string `json:"description"`
	Hours       uint   `json:"hours"`
	Minutes     uint   `json:"minutes"`
	Seconds     uint   `json:"seconds"`
}

func (r AddBookmarkRequest) ToBookmark(userID uint) model.Bookmark {
	return model.Bookmark{
		Description: r.Description,
		Hours:       r.Hours,
		Minutes:     r.Minutes,
		Seconds:     r.Seconds,
		StreamID:    r.StreamID,
		UserID:      userID,
	}
}

func (r bookmarkRoutes) Add(c *gin.Context) {
	var err error
	var req AddBookmarkRequest
	var user *model.User
	var bookmark model.Bookmark

	err = c.BindJSON(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark = req.ToBookmark(user.ID)
	err = r.BookmarkDao.Add(&bookmark)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can add bookmark",
			Err:           err,
		})
		return
	}
}

type GetBookmarksQuery struct {
	StreamID uint `form:"streamID" binding:"required"`
}

func (r bookmarkRoutes) GetByStreamID(c *gin.Context) {
	var err error
	var query GetBookmarksQuery
	var user *model.User
	var bookmarks []model.Bookmark

	err = c.BindQuery(&query)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind query",
			Err:           err,
		})
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmarks, err = r.BookmarkDao.GetByStreamID(query.StreamID, user.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusNotFound,
				CustomMessage: "invalid stream",
			})
			return
		}
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get bookmarks",
			Err:           err,
		})
		return
	}

	c.JSON(http.StatusOK, bookmarks)
}

type UpdateBookmarkRequest struct {
	Description string `json:"description"`
	Hours       uint   `json:"hours"`
	Minutes     uint   `json:"minutes"`
	Seconds     uint   `json:"seconds"`
}

func (r UpdateBookmarkRequest) ToBookmark(id uint) model.Bookmark {
	return model.Bookmark{
		Model:       gorm.Model{ID: id},
		Description: r.Description,
		Hours:       r.Hours,
		Minutes:     r.Minutes,
		Seconds:     r.Seconds,
	}
}

func (r bookmarkRoutes) Update(c *gin.Context) {
	var err error
	var req UpdateBookmarkRequest
	var user *model.User
	var bookmark model.Bookmark
	var id int

	id, err = strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid id",
			Err:           err,
		})
		return
	}

	err = c.BindJSON(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark, err = r.BookmarkDao.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusNotFound,
				CustomMessage: "invalid bookmark id",
				Err:           err,
			})
			return
		}
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get bookmarks by id",
			Err:           err,
		})
		return
	}

	if bookmark.UserID != user.ID {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "logged in user is not the creator of the bookmark",
		})
		return
	}

	bookmark = req.ToBookmark(uint(id))
	err = r.BookmarkDao.Update(&bookmark)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update bookmark",
			Err:           err,
		})
		return
	}
}

func (r bookmarkRoutes) Delete(c *gin.Context) {
	var err error
	var user *model.User
	var bookmark model.Bookmark
	var id int

	id, err = strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid id",
			Err:           err,
		})
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark, err = r.BookmarkDao.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusNotFound,
				CustomMessage: "invalid bookmark id",
				Err:           err,
			})
			return
		}
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get bookmarks by id",
			Err:           err,
		})
		return
	}

	if bookmark.UserID != user.ID {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "logged in user is not the creator of the bookmark",
		})
		return
	}

	err = r.BookmarkDao.Delete(uint(id))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete bookmark",
			Err:           err,
		})
		return
	}
}
