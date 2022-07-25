package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"net/http"
	"testing"
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
				Router:       ReportSeekRouterWrapper,
				Method:       http.MethodPost,
				Url:          fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
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
				Method:       http.MethodPost,
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
				Method:       http.MethodPost,
				Url:          fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
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
				Method:       http.MethodPost,
				Url:          fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
				Body:         body,
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, testutils.Equal)
	})

	response := gin.H{
		"values": []gin.H{
			{
				"index": testutils.FPVNotLiveVideoSeekChunk1.ChunkIndex,
				"value": testutils.FPVNotLiveVideoSeekChunk1.Hits,
			},
			{
				"index": testutils.FPVNotLiveVideoSeekChunk2.ChunkIndex,
				"value": testutils.FPVNotLiveVideoSeekChunk2.Hits,
			},
			{
				"index": testutils.FPVNotLiveVideoSeekChunk3.ChunkIndex,
				"value": testutils.FPVNotLiveVideoSeekChunk3.Hits,
			},
		},
	}

	t.Run("GET/api/seekReport/:streamID", func(t *testing.T) {
		baseUrl := "/api/seekReport"

		ctrl := gomock.NewController(t)

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
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
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
				Method:           http.MethodGet,
				Url:              fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
				ExpectedResponse: response,
				ExpectedCode:     http.StatusOK,
			},
		}.Run(t, testutils.Equal)
	})
}
