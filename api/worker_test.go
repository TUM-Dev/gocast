package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE[DeleteWorker returns error]", func(t *testing.T) {
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
		workerDaoMock.EXPECT().DeleteWorker(workerId).Return(errors.New("")).AnyTimes()

		configWorkerRouter(r, dao.DaoWrapper{WorkerDao: workerDaoMock})

		c.Request, _ = http.NewRequest(http.MethodDelete, "/api/workers/"+workerId, nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DELETE[success]", func(t *testing.T) {
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

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
