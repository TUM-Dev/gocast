package api

import (
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
)

func TestGetAudits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// audit mock
	mock := mock_dao.NewMockAuditDao(gomock.NewController(t))
	mock.EXPECT().Find(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.Audit{}, nil).AnyTimes()
	mock.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()

	gomino.TestCases{
		"get audits": {
			Router: func(r *gin.Engine) {
				wrapper := dao.DaoWrapper{AuditDao: mock}
				configAuditRouter(r, wrapper)
			},
			Method:       http.MethodGet,
			Url:          "/api/audits?limit=1&offset=0&types[]=1",
			Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
			ExpectedCode: http.StatusOK,
		},
	}.Run(t, testutils.Equal)
}
