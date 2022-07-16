package api

import (
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

type AddRequest struct {
	StreamID    uint   `json:"streamID"`
	Description string `json:"description"`
	Hours       uint   `json:"hours"`
	Minutes     uint   `json:"minutes"`
	Seconds     uint   `json:"seconds"`
}

func (r AddRequest) ToBookmark(userID uint) model.Bookmark {
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
	var req AddRequest
	var user *model.User
	var bookmark model.Bookmark

	err = c.BindJSON(&req)
	if err != nil {
		// TODO: New Error handling
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark = req.ToBookmark(user.ID)
	err = r.BookmarkDao.Add(&bookmark)
	if err != nil {
		// TODO: New Error handling
		return
	}
}

type GetQuery struct {
	StreamID uint `form:"streamID"`
}

func (r bookmarkRoutes) GetByStreamID(c *gin.Context) {
	var err error
	var query GetQuery
	var user *model.User
	var bookmarks []model.Bookmark

	err = c.BindQuery(&query)
	if err != nil {
		// TODO: New Error handling
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmarks, err = r.BookmarkDao.GetByStreamID(query.StreamID, user.ID)
	if err != nil {
		// TODO: New Error handling
		return
	}

	c.JSON(http.StatusOK, bookmarks)
}

type UpdateRequest struct {
	Description string `json:"description"`
	Hours       uint   `json:"hours"`
	Minutes     uint   `json:"minutes"`
	Seconds     uint   `json:"seconds"`
}

func (r UpdateRequest) ToBookmark(id uint) model.Bookmark {
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
	var req UpdateRequest
	var user *model.User
	var bookmark model.Bookmark
	var id int

	id, err = strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: New Error handling
		return
	}

	err = c.BindJSON(&req)
	if err != nil {
		// TODO: New Error handling
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark, err = r.BookmarkDao.GetByID(uint(id))
	if err != nil {
		// TODO: New Error handling
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
		return
	}

	user = c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	bookmark, err = r.BookmarkDao.GetByID(uint(id))
	if err != nil {
		// TODO: New Error handling
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
		return
	}
}
