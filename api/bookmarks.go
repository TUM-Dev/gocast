package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"gorm.io/gorm"
	"net/http"
	"strconv"
)

func configGinBookmarksRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := bookmarkRoutes{daoWrapper}
	bookmarks := router.Group("/api/bookmarks")
	{
		//bookmarks.Use(tools.InitContext(daoWrapper))
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
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark = req.ToBookmark(user.ID)
	err = r.BookmarkDao.Add(&bookmark)
	if err != nil {
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusInternalServerError)
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
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmarks, err = r.BookmarkDao.GetByStreamID(query.StreamID, user.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
		// TODO: New Error handling
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
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	err = c.BindJSON(&req)
	if err != nil {
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark, err = r.BookmarkDao.GetByID(uint(id))
	if err != nil {
		// TODO: New Error handling
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if bookmark.UserID != user.ID {
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	bookmark = req.ToBookmark(uint(id))
	err = r.BookmarkDao.Update(&bookmark)
	if err != nil {
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusBadRequest)
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
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark, err = r.BookmarkDao.GetByID(uint(id))
	if err != nil {
		// TODO: New Error handling
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if bookmark.UserID != user.ID {
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	err = r.BookmarkDao.Delete(uint(id))
	if err != nil {
		// TODO: New Error handling
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
