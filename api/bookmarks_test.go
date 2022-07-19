package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"gorm.io/gorm"
	"net/http"
	"testing"
)

func TestBookmarks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/bookmarks", func(t *testing.T) {
		url := "/api/bookmarks"

		req := AddBookmarkRequest{
			StreamID:    testutils.StreamFPVLive.ID,
			Description: "klausurrelevant",
			Hours:       1,
			Minutes:     33,
			Seconds:     7,
		}

		bookmark := req.ToBookmark(testutils.Student.ID)

		testutils.TestCases{
			"not logged in": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextEmpty,
				ExpectedCode:   http.StatusFound,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				Body:           nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"can not add bookmark": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							Add(&bookmark).
							Return(errors.New("")).
							AnyTimes()
						return mock
					}(),
				},
				Body:         req,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							Add(&bookmark).
							Return(nil).
							AnyTimes()
						return mock
					}(),
				},
				Body:         req,
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, configGinBookmarksRouter)
	})
	t.Run("GET/api/bookmarks", func(t *testing.T) {
		baseUrl := "/api/bookmarks"

		bookmarks := []model.Bookmark{testutils.Bookmark}

		testutils.TestCases{
			"not logged in": {
				Method:         http.MethodGet,
				Url:            baseUrl,
				TumLiveContext: &testutils.TUMLiveContextEmpty,
				ExpectedCode:   http.StatusFound,
			},
			"invalid query": {
				Method:         http.MethodGet,
				Url:            baseUrl,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusBadRequest,
			},
			"can not find stream": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?streamID=%d", baseUrl, testutils.StreamFPVLive.ID),
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByStreamID(testutils.StreamFPVLive.ID, testutils.Student.ID).
							Return([]model.Bookmark{}, gorm.ErrRecordNotFound).
							AnyTimes()
						return mock
					}(),
				},
				Body:         nil,
				ExpectedCode: http.StatusNotFound,
			},
			"database error": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?streamID=%d", baseUrl, testutils.StreamFPVLive.ID),
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByStreamID(testutils.StreamFPVLive.ID, testutils.Student.ID).
							Return([]model.Bookmark{}, errors.New("")).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?streamID=%d", baseUrl, testutils.StreamFPVLive.ID),
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByStreamID(testutils.StreamFPVLive.ID, testutils.Student.ID).
							Return(bookmarks, nil).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(bookmarks)).([]byte),
			},
		}.Run(t, configGinBookmarksRouter)
	})
	t.Run("PUT/api/bookmarks/:id", func(t *testing.T) {
		url := fmt.Sprintf("/api/bookmarks/%d", testutils.Bookmark.ID)

		req := UpdateBookmarkRequest{
			Description: "Klausurrelevant!",
			Hours:       1,
			Minutes:     33,
			Seconds:     7,
		}

		updatedBookmark := req.ToBookmark(testutils.Bookmark.ID)

		testutils.TestCases{
			"not logged in": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextEmpty,
				ExpectedCode:   http.StatusFound,
			},
			"invalid id": {
				Method:         http.MethodPut,
				Url:            "/api/bookmarks/abc",
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid body": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				Body:           nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"can not find bookmark": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, gorm.ErrRecordNotFound).
							AnyTimes()
						return mock
					}(),
				},
				Body:         req,
				ExpectedCode: http.StatusNotFound,
			},
			"can not find bookmark - database error": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, errors.New("")).
							AnyTimes()
						return mock
					}(),
				},
				Body:         req,
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid user": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextLecturer,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, nil).
							AnyTimes()
						return mock
					}(),
				},
				Body:         req,
				ExpectedCode: http.StatusForbidden,
			},
			"can not update bookmark": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, nil).
							AnyTimes()
						mock.
							EXPECT().
							Update(&updatedBookmark).
							Return(errors.New("")).
							AnyTimes()
						return mock
					}(),
				},
				Body:         req,
				ExpectedCode: http.StatusBadRequest,
			},
			"success": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, nil).
							AnyTimes()
						mock.
							EXPECT().
							Update(&updatedBookmark).
							Return(nil).
							AnyTimes()
						return mock
					}(),
				},
				Body:         req,
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, configGinBookmarksRouter)
	})
	t.Run("DELETE/api/bookmarks/:id", func(t *testing.T) {
		url := fmt.Sprintf("/api/bookmarks/%d", testutils.Bookmark.ID)

		testutils.TestCases{
			"not logged in": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextEmpty,
				ExpectedCode:   http.StatusFound,
			},
			"invalid id": {
				Method:         http.MethodDelete,
				Url:            "/api/bookmarks/abc",
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusBadRequest,
			},
			"can not find bookmark": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, gorm.ErrRecordNotFound).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode: http.StatusNotFound,
			},
			"can not find bookmark - database error": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, errors.New("")).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid user": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextLecturer,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, nil).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"can not delete bookmark": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, nil).
							AnyTimes()
						mock.
							EXPECT().
							Delete(testutils.Bookmark.ID).
							Return(errors.New("")).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					BookmarkDao: func() dao.BookmarkDao {
						mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
						mock.
							EXPECT().
							GetByID(testutils.Bookmark.ID).
							Return(testutils.Bookmark, nil).
							AnyTimes()
						mock.
							EXPECT().
							Delete(testutils.Bookmark.ID).
							Return(nil).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, configGinBookmarksRouter)
	})
}
