package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
)

func TestProgressReport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/progressReport", func(t *testing.T) {
		url := "/api/progressReport"

		req := progressRequest{
			StreamID: uint(1),
			Progress: 0,
		}

		testCases := testutils.TestCases{
			"invalid body": {
				Method:       http.MethodPost,
				Url:          url,
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"no context": {
				Method:       http.MethodPost,
				Url:          url,
				Body:         req,
				ExpectedCode: http.StatusBadRequest,
			},
			"not logged in": {
				Method:         http.MethodPost,
				Url:            url,
				Body:           req,
				TumLiveContext: &testutils.TUMLiveContextUserNil,
				ExpectedCode:   http.StatusForbidden,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				Body:           req,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusOK,
			},
		}

		testCases.Run(t, configProgressRouter)
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

		testCases := testutils.TestCases{
			"invalid body": {
				Method:       http.MethodPost,
				Url:          url,
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				Body:           req,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"not logged in": {
				Method:         http.MethodPost,
				Url:            url,
				Body:           req,
				TumLiveContext: &testutils.TUMLiveContextUserNil,
				ExpectedCode:   http.StatusForbidden,
			},
			"can not save progress": {
				Method:         http.MethodPost,
				Url:            url,
				Body:           req,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					ProgressDao: func() dao.ProgressDao {
						progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
						progressMock.
							EXPECT().
							SaveWatchedState(gomock.Any()).
							Return(errors.New(""))
						return progressMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				Body:           req,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					ProgressDao: func() dao.ProgressDao {
						progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
						progressMock.
							EXPECT().
							SaveWatchedState(gomock.Any()).
							Return(nil)
						return progressMock
					}(),
				},
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configProgressRouter)
	})
}
