package api

import (
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/magiconair/properties/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeleteWorker_success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	workerId := "1234"

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	// Middleware to set Mock-TUMLiveContext
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
			Name: "Admin",
			Role: model.AdminType,
		}})
	})

	ctrl := gomock.NewController(t)
	workerDaoMock := mock_dao.NewMockWorkerDao(ctrl)
	workerDaoMock.EXPECT().DeleteWorker(workerId).Return(nil).AnyTimes()

	configWorkerRouter(r, dao.DaoWrapper{WorkerDao: workerDaoMock})

	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/workers/"+workerId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, w.Code, 200)
}
