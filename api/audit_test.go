package api

import (
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAudits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// audit mock
	mock := mock_dao.NewMockAuditDao(gomock.NewController(t))
	mock.EXPECT().Find(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.Audit{}, nil).AnyTimes()

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	configAuditRouter(r, dao.DaoWrapper{})
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/audits/?limit=1&offset=0&types[]=1", nil)
	r.ServeHTTP(w, c.Request)

	t.Log(w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)

}
