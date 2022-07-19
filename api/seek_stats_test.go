package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
)

func TestReportSeek(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/seekReport/:streamID", func(t *testing.T) {
		baseUrl := "/api/seekReport"

		ctrl := gomock.NewController(t)

		testPosition := 120.32
		testBody := testutils.First(json.Marshal(gin.H{
			"position": testPosition,
		})).([]byte)

		testutils.TestCases{
			"missing position": {
				Method:       http.MethodPost,
				Url:          fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid courseId": {
				Method: http.MethodPost,
				Url:    fmt.Sprintf("%s/abc", baseUrl),
				Body:   bytes.NewBuffer(testBody),
				DaoWrapper: dao.DaoWrapper{
					VideoSeekDao: func() dao.VideoSeekDao {
						searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
						searchMock.
							EXPECT().
							Add("abc", testPosition).
							Return(errors.New(""))
						return searchMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not add seek record": {
				Method: http.MethodPost,
				Url:    fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
				Body:   bytes.NewBuffer(testBody),
				DaoWrapper: dao.DaoWrapper{
					VideoSeekDao: func() dao.VideoSeekDao {
						searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
						searchMock.
							EXPECT().
							Add(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID), testPosition).
							Return(errors.New(""))
						return searchMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method: http.MethodPost,
				Url:    fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
				Body:   bytes.NewBuffer(testBody),
				DaoWrapper: dao.DaoWrapper{
					VideoSeekDao: func() dao.VideoSeekDao {
						searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
						searchMock.
							EXPECT().
							Add(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID), testPosition).
							Return(nil)
						return searchMock
					}(),
				},
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, configSeekStatsRouter)
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

		testutils.TestCases{
			"failed to read video seek chunks": {
				Method: http.MethodGet,
				Url:    fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
				DaoWrapper: dao.DaoWrapper{
					VideoSeekDao: func() dao.VideoSeekDao {
						searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
						searchMock.
							EXPECT().
							Get(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID)).
							Return(nil, errors.New(""))
						return searchMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method: http.MethodGet,
				Url:    fmt.Sprintf("%s/%d", baseUrl, testutils.StreamFPVNotLive.ID),
				DaoWrapper: dao.DaoWrapper{
					VideoSeekDao: func() dao.VideoSeekDao {
						searchMock := mock_dao.NewMockVideoSeekDao(ctrl)
						searchMock.
							EXPECT().
							Get(fmt.Sprintf("%d", testutils.StreamFPVNotLive.ID)).
							Return([]model.VideoSeekChunk{testutils.FPVNotLiveVideoSeekChunk1, testutils.FPVNotLiveVideoSeekChunk2, testutils.FPVNotLiveVideoSeekChunk3}, nil)
						return searchMock
					}(),
				},
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
				ExpectedCode:     http.StatusOK,
			},
		}.Run(t, configSeekStatsRouter)
	})
}
