package api

import (
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
)

func TestGetAudits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// audit mock
	mock := mock_dao.NewMockAuditDao(gomock.NewController(t))
	mock.EXPECT().Find(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.Audit{}, nil).AnyTimes()
	mock.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()

	testCases := testutils.TestCases{
		"get audits": {
			Method:         http.MethodGet,
			Url:            "/api/audits?limit=1&offset=0&types[]=1",
			DaoWrapper:     dao.DaoWrapper{AuditDao: mock},
			TumLiveContext: &testutils.TUMLiveContextAdmin,
			ExpectedCode:   http.StatusOK,
		},
	}
	testCases.Run(t, configAuditRouter)
}
