package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
)

func TestWorker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE/api/workers/:workerID", func(t *testing.T) {
		url := fmt.Sprintf("/api/workers/%s", testutils.Worker1.WorkerID)
		testCases := testutils.TestCases{
			"can not delete worker": {
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					WorkerDao: func() dao.WorkerDao {
						workerDaoMock := mock_dao.NewMockWorkerDao(gomock.NewController(t))
						workerDaoMock.
							EXPECT().
							DeleteWorker(testutils.Worker1.WorkerID).
							Return(errors.New("")).
							AnyTimes()
						return workerDaoMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"success": {
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					WorkerDao: func() dao.WorkerDao {
						workerDaoMock := mock_dao.NewMockWorkerDao(gomock.NewController(t))
						workerDaoMock.
							EXPECT().
							DeleteWorker(testutils.Worker1.WorkerID).
							Return(nil).
							AnyTimes()
						return workerDaoMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
			},
		}

		testCases.Run(t, configWorkerRouter)
	})
}
