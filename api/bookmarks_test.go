package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/matthiasreumann/gomino"
	"gorm.io/gorm"
)

func BookmarksRouterWrapper(r *gin.Engine) {
	configGinBookmarksRouter(r, dao.DaoWrapper{})
}

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

		gomino.TestCases{
			"not logged in": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
				ExpectedCode: http.StatusFound,
			},
			"invalid body": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"can not add bookmark": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								Add(&bookmark).
								Return(errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         req,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								Add(&bookmark).
								Return(nil).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         req,
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, testutils.Equal)
	})
	t.Run("GET/api/bookmarks", func(t *testing.T) {
		baseUrl := "/api/bookmarks"

		bookmarks := []model.Bookmark{testutils.Bookmark}

		gomino.TestCases{
			"not logged in": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodGet,
				Url:          baseUrl,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
				ExpectedCode: http.StatusFound,
			},
			"invalid query": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodGet,
				Url:          baseUrl,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find stream": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByStreamID(testutils.StreamFPVLive.ID, testutils.Student.ID).
								Return([]model.Bookmark{}, gorm.ErrRecordNotFound).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("%s?streamID=%d", baseUrl, testutils.StreamFPVLive.ID),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         nil,
				ExpectedCode: http.StatusNotFound,
			},
			"database error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByStreamID(testutils.StreamFPVLive.ID, testutils.Student.ID).
								Return([]model.Bookmark{}, errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("%s?streamID=%d", baseUrl, testutils.StreamFPVLive.ID),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByStreamID(testutils.StreamFPVLive.ID, testutils.Student.ID).
								Return(bookmarks, nil).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:           http.MethodGet,
				Url:              fmt.Sprintf("%s?streamID=%d", baseUrl, testutils.StreamFPVLive.ID),
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: bookmarks,
			},
		}.Run(t, testutils.Equal)
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

		gomino.TestCases{
			"not logged in": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodPut,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
				ExpectedCode: http.StatusFound,
			},
			"invalid id": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodPut,
				Url:          "/api/bookmarks/abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid body": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodPut,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find bookmark": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByID(testutils.Bookmark.ID).
								Return(testutils.Bookmark, gorm.ErrRecordNotFound).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodPut,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         req,
				ExpectedCode: http.StatusNotFound,
			},
			"can not find bookmark - database error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByID(testutils.Bookmark.ID).
								Return(testutils.Bookmark, errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodPut,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         req,
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid user": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByID(testutils.Bookmark.ID).
								Return(testutils.Bookmark, nil).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodPut,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturer)),
				Body:         req,
				ExpectedCode: http.StatusForbidden,
			},
			"can not update bookmark": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodPut,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         req,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodPut,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				Body:         req,
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, testutils.Equal)
	})
	t.Run("DELETE/api/bookmarks/:id", func(t *testing.T) {
		url := fmt.Sprintf("/api/bookmarks/%d", testutils.Bookmark.ID)

		gomino.TestCases{
			"not logged in": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodDelete,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
				ExpectedCode: http.StatusFound,
			},
			"invalid id": {
				Router:       BookmarksRouterWrapper,
				Method:       http.MethodDelete,
				Url:          "/api/bookmarks/abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find bookmark": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByID(testutils.Bookmark.ID).
								Return(testutils.Bookmark, gorm.ErrRecordNotFound).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodDelete,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusNotFound,
			},
			"can not find bookmark - database error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByID(testutils.Bookmark.ID).
								Return(testutils.Bookmark, errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodDelete,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid user": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						BookmarkDao: func() dao.BookmarkDao {
							mock := mock_dao.NewMockBookmarkDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetByID(testutils.Bookmark.ID).
								Return(testutils.Bookmark, nil).
								AnyTimes()
							return mock
						}(),
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodDelete,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturer)),
				ExpectedCode: http.StatusForbidden,
			},
			"can not delete bookmark": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodDelete,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinBookmarksRouter(r, wrapper)
				},
				Method:       http.MethodDelete,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, testutils.Equal)
	})
}
