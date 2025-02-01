package apiv2

import (
	"context"
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

// AddBookmark adds a new bookmark.
func (a *API) AddBookmark(ctx context.Context, req *protobuf.AddBookmarkRequest) (*protobuf.AddBookmarkResponse, error) {
	a.log.Info("AddBookmark")

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	bookmark := model.Bookmark{
		Description: req.Description,
		Hours:       uint(req.Hours),
		Minutes:     uint(req.Minutes),
		Seconds:     uint(req.Seconds),
		StreamID:    uint(req.StreamId),
		UserID:      user.ID,
	}

	err = a.dao.BookmarkDao.Add(&bookmark)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.AddBookmarkResponse{Bookmark: h.ParseBookmarkToProto(bookmark)}, nil
}

// GetBookmarks retrieves bookmarks by stream ID.
func (a *API) GetBookmarks(ctx context.Context, req *protobuf.GetBookmarksRequest) (*protobuf.GetBookmarksResponse, error) {
	a.log.Info("GetBookmarks")

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	bookmarks, err := a.dao.BookmarkDao.GetByStreamID(uint(req.StreamId), user.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &protobuf.GetBookmarksResponse{}, nil
		}
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := &protobuf.GetBookmarksResponse{Bookmarks: make([]*protobuf.Bookmark, len(bookmarks))}
	for i, bookmark := range bookmarks {
		resp.Bookmarks[i] = h.ParseBookmarkToProto(bookmark)
	}

	return resp, nil
}

// UpdateBookmark updates an existing bookmark.
func (a *API) UpdateBookmark(ctx context.Context, req *protobuf.UpdateBookmarkRequest) (*protobuf.UpdateBookmarkResponse, error) {
	a.log.Info("UpdateBookmark")

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	bookmark, err := a.dao.BookmarkDao.GetByID(uint(req.Id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.WithStatus(http.StatusNotFound, errors.New("Invalid bookmark ID"))
		}
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if bookmark.UserID != user.ID {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("Logged in user is not the creator of the bookmark"))
	}

	bookmark.Description = req.Description
	bookmark.Hours = uint(req.Hours)
	bookmark.Minutes = uint(req.Minutes)
	bookmark.Seconds = uint(req.Seconds)

	err = a.dao.BookmarkDao.Update(&bookmark)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.UpdateBookmarkResponse{Bookmark: h.ParseBookmarkToProto(bookmark)}, nil
}

// DeleteBookmark deletes a bookmark.
func (a *API) DeleteBookmark(ctx context.Context, req *protobuf.DeleteBookmarkRequest) (*emptypb.Empty, error) {
	a.log.Info("DeleteBookmark")

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	bookmark, err := a.dao.BookmarkDao.GetByID(uint(req.Id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.WithStatus(http.StatusNotFound, errors.New("Invalid bookmark ID"))
		}
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if bookmark.UserID != user.ID {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("Logged in user is not the creator of the bookmark"))
	}

	err = a.dao.BookmarkDao.Delete(uint(req.Id))
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &emptypb.Empty{}, nil
}
