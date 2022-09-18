package api

import (
	"errors"
	"fmt"
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

func TestWorker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE/api/workers/:workerID", func(t *testing.T) {
		url := fmt.Sprintf("/api/workers/%s", testutils.Worker1.WorkerID)
		gomino.TestCases{
			"can not delete worker": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						WorkerDao: func() dao.WorkerDao {
							workerDaoMock := mock_dao.NewMockWorkerDao(gomock.NewController(t))
							workerDaoMock.
								EXPECT().
								DeleteWorker(testutils.Worker1.WorkerID).
								Return(errors.New("")).
								AnyTimes()
							return workerDaoMock
						}(),
					}
					configWorkerRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						WorkerDao: func() dao.WorkerDao {
							workerDaoMock := mock_dao.NewMockWorkerDao(gomock.NewController(t))
							workerDaoMock.
								EXPECT().
								DeleteWorker(testutils.Worker1.WorkerID).
								Return(nil).
								AnyTimes()
							return workerDaoMock
						}(),
					}
					configWorkerRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodDelete).
			Url(url).
			Run(t, testutils.Equal)
	})
}
