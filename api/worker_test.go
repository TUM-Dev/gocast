package api

import (
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/magiconair/properties/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createMock(t *testing.T, workerId string) *mock_dao.MockWorkerDao {
	ctrl := gomock.NewController(t)
	workerDaoMock := mock_dao.NewMockWorkerDao(ctrl)
	workerDaoMock.EXPECT().DeleteWorker(workerId).Return(nil).AnyTimes()

	return workerDaoMock
}

func createMockTUMLiveContext() tools.TUMLiveContext {
	return tools.TUMLiveContext{User: &model.User{
		Name: "Admin",
		Role: model.AdminType,
	}}
}

func TestDeleteWorker_success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	workerId := "1234"
	workerDaoMock := createMock(t, workerId)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	// Middleware to set Mock-TUMLiveContext
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", createMockTUMLiveContext())
	})

	configWorkerRouter(r, workerDaoMock)

	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/workers/"+workerId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, w.Code, 200)
	assert.Equal(t, w.Body.String(), "")
}
