package api

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/matthiasreumann/gomino"
)

func TokenRouterWrapper(r *gin.Engine) {
	configTokenRouter(r, dao.DaoWrapper{})
}

func TestToken(t *testing.T) {
	t.Run("/create", func(t *testing.T) {
		url := "/api/token/create"

		now := time.Now()
		type req struct {
			Expires *time.Time `json:"expires"`
			Scope   string     `json:"scope"`
		}
		gomino.TestCases{
			"POST[No Context]": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[Invalid Body]": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[Invalid Scope]": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         req{Expires: &now, Scope: "invalid"},
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[AddToken returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						TokenDao: func() dao.TokenDao {
							tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
							tokenMock.EXPECT().AddToken(gomock.Any()).Return(errors.New("")).AnyTimes()
							return tokenMock
						}(),
					}
					configTokenRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         req{Expires: &now, Scope: model.TokenScopeAdmin},
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[success]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						TokenDao: func() dao.TokenDao {
							tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
							tokenMock.EXPECT().AddToken(gomock.Any()).Return(nil).AnyTimes()
							return tokenMock
						}(),
					}
					configTokenRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         req{Expires: &now, Scope: model.TokenScopeAdmin},
				ExpectedCode: http.StatusOK,
			},
		}.
			Router(TokenRouterWrapper).
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})

	t.Run("/:id", func(t *testing.T) {
		url := "/api/token/1"
		gomino.TestCases{
			"DELETE[DeleteToken returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						TokenDao: func() dao.TokenDao {
							tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
							tokenMock.EXPECT().DeleteToken("1").Return(errors.New("")).AnyTimes()
							return tokenMock
						}(),
					}
					configTokenRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"DELETE[Success]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						TokenDao: func() dao.TokenDao {
							tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
							tokenMock.EXPECT().DeleteToken("1").Return(nil).AnyTimes()
							return tokenMock
						}(),
					}
					configTokenRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			},
		}.
			Method(http.MethodDelete).
			Url(url).
			Run(t, testutils.Equal)
	})
}
