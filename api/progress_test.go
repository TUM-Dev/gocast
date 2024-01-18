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

func ProgressRouterWrapper(r *gin.Engine) {
	configProgressRouter(r, dao.DaoWrapper{})
}

func TestProgressReport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/progressReport", func(t *testing.T) {
		url := "/api/progressReport"

		req := progressRequest{
			StreamID: uint(1),
			Progress: 0,
		}

		gomino.TestCases{
			"invalid body": {
				Router:       ProgressRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusBadRequest,
			},
			"no context": {
				Router:       ProgressRouterWrapper,
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusBadRequest,
			},
			"not logged in": {
				Router:       ProgressRouterWrapper,
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusForbidden,
			},
			"success": {
				Router:       ProgressRouterWrapper,
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusOK,
			},
		}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestWatched(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/watched", func(t *testing.T) {
		url := "/api/watched"

		req := watchedRequest{
			StreamID: testutils.StreamFPVLive.ID,
			Watched:  true,
		}

		gomino.TestCases{
			"no context": {
				Router:       ProgressRouterWrapper,
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusBadRequest,
			},
			"not logged in": {
				Router:       ProgressRouterWrapper,
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Router:       ProgressRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not save progress": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						ProgressDao: func() dao.ProgressDao {
							progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
							progressMock.
								EXPECT().
								SaveWatchedState(gomock.Any()).
								Return(errors.New(""))
							return progressMock
						}(),
					}
					configProgressRouter(r, wrapper)
				},
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						ProgressDao: func() dao.ProgressDao {
							progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
							progressMock.
								EXPECT().
								SaveWatchedState(gomock.Any()).
								Return(nil)
							return progressMock
						}(),
					}
					configProgressRouter(r, wrapper)
				},
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusOK,
			},
		}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestUserProgress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/progress/streams", func(t *testing.T) {
		url := "/api/progress/streams"

		gomino.TestCases{
			"not logged in": {
				Router:       ProgressRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid query": {
				Router:       ProgressRouterWrapper,
				Url:          fmt.Sprintf("%s?[]wrong=1", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can't get stream progress": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						ProgressDao: func() dao.ProgressDao {
							progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
							progressMock.
								EXPECT().
								LoadProgress(testutils.TUMLiveContextStudent.User.ID, gomock.Any()).
								Return([]model.StreamProgress{}, errors.New(""))
							return progressMock
						}(),
					}
					configProgressRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("%s?[]ids=16", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success skip not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						ProgressDao: func() dao.ProgressDao {
							progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
							progressMock.
								EXPECT().
								LoadProgress(testutils.TUMLiveContextStudent.User.ID, gomock.Any()).
								Return([]model.StreamProgress{}, gorm.ErrRecordNotFound)
							return progressMock
						}(),
					}
					configProgressRouter(r, wrapper)
				},
				Url:              fmt.Sprintf("%s?[]ids=16", url),
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []model.StreamProgress{{StreamID: 16}},
			},
			"success skip invalid": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						ProgressDao: func() dao.ProgressDao {
							progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
							progressMock.
								EXPECT().
								LoadProgress(testutils.TUMLiveContextStudent.User.ID, gomock.Any()).
								Return([]model.StreamProgress{{StreamID: 16, Watched: true, Progress: 0.5}}, nil).
								AnyTimes()
							return progressMock
						}(),
					}
					configProgressRouter(r, wrapper)
				},
				Url:              fmt.Sprintf("%s?[]ids=16&[]ids=XYZ", url),
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []model.StreamProgress{{StreamID: 16, Watched: true, Progress: 0.5}},
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						ProgressDao: func() dao.ProgressDao {
							progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
							progressMock.
								EXPECT().
								LoadProgress(testutils.TUMLiveContextStudent.User.ID, gomock.Any()).
								Return([]model.StreamProgress{{StreamID: 16, Watched: true, Progress: 0.5}}, nil)
							return progressMock
						}(),
					}
					configProgressRouter(r, wrapper)
				},
				Url:              fmt.Sprintf("%s?[]ids=16", url),
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []model.StreamProgress{{StreamID: 16, Watched: true, Progress: 0.5}},
			},
		}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
}
