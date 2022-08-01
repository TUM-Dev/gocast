package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"net/http"
	"testing"
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
			}}.
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
			"invalid body": {
				Router:       ProgressRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusBadRequest,
			},
			"no context": {
				Router:       ProgressRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				Body:         req,
				ExpectedCode: http.StatusBadRequest,
			},
			"not logged in": {
				Router:       ProgressRouterWrapper,
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusForbidden,
			},
			"can not save progress": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						ProgressDao: func() dao.ProgressDao {
							progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
							progressMock.
								EXPECT().
								SaveWatchedState(gomock.Any(), gomock.Any()).
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
								SaveWatchedState(gomock.Any(), gomock.Any()).
								Return(nil)
							return progressMock
						}(),
					}
					configProgressRouter(r, wrapper)
				},
				Body:         req,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}
