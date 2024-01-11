package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/matthiasreumann/gomino"
)

func ReportSeekRouterWrapper(r *gin.Engine) {
	configSeekStatsRouter(r, dao.DaoWrapper{})
}

func TestReportSeek(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/seekReport/:streamID", func(t *testing.T) {
		baseUrl := "/api/seekReport"

		ctrl := gomock.NewController(t)

		testPosition := 120.32
		body := gin.H{"position": testPosition}

		gomino.TestCases{
			"missing position": {
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid courseId": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						VideoSeekDao: func() dao.VideoSeekDao {
							searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
							searchMock.
								EXPECT().
								Add("abc", testPosition).
								Return(errors.New(""))
							return searchMock
						}(),
					}
					configSeekStatsRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("%s/abc", baseUrl),
				Body:         body,
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not add seek record": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						VideoSeekDao: func() dao.VideoSeekDao {
							searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
							searchMock.
								EXPECT().
								Add(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID), testPosition).
								Return(errors.New(""))
							return searchMock
						}(),
					}
					configSeekStatsRouter(r, wrapper)
				},
				Body:         body,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						VideoSeekDao: func() dao.VideoSeekDao {
							searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
							searchMock.
								EXPECT().
								Add(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID), testPosition).
								Return(nil)
							return searchMock
						}(),
					}
					configSeekStatsRouter(r, wrapper)
				},
				Body:         body,
				ExpectedCode: http.StatusOK,
			}}.
			Router(ReportSeekRouterWrapper).
			Method(http.MethodPost).
			Url(fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID)).
			Run(t, testutils.Equal)
	})

	emptyResponse := gin.H{
		"values": []gin.H{},
	}

	t.Run("GET/api/seekReport/:streamID", func(t *testing.T) {
		baseUrl := "/api/seekReport"

		ctrl := gomock.NewController(t)

		testChunks, testResponse := testutils.CreateVideoSeekData(testutils.FPVNotLiveVideoSeekChunk1.StreamID, 50)

		gomino.TestCases{
			"failed to read video seek chunks": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						VideoSeekDao: func() dao.VideoSeekDao {
							searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
							searchMock.
								EXPECT().
								Get(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID)).
								Return(nil, errors.New(""))
							return searchMock
						}(),
					}
					configSeekStatsRouter(r, wrapper)
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"empty because of not enough seek data": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						VideoSeekDao: func() dao.VideoSeekDao {
							searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
							searchMock.
								EXPECT().
								Get(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID)).
								Return([]model.VideoSeekChunk{testutils.FPVNotLiveVideoSeekChunk1, testutils.FPVNotLiveVideoSeekChunk2, testutils.FPVNotLiveVideoSeekChunk3}, nil)
							return searchMock
						}(),
					}
					configSeekStatsRouter(r, wrapper)
				},
				ExpectedResponse: emptyResponse,
				ExpectedCode:     http.StatusOK,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						VideoSeekDao: func() dao.VideoSeekDao {
							searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
							searchMock.
								EXPECT().
								Get(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID)).
								Return(testChunks, nil)
							return searchMock
						}(),
					}
					configSeekStatsRouter(r, wrapper)
				},
				ExpectedResponse: testResponse,
				ExpectedCode:     http.StatusOK,
			}}.
			Method(http.MethodGet).
			Url(fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID)).
			Run(t, testutils.Equal)
	})
}
