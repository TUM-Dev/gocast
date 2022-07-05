package api

import (
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

func TestSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/search/streams", func(t *testing.T) {
		baseUrl := "/api/search/streams"

		ctrl := gomock.NewController(t)

		queryString := "klausurrelevant"

		response := gin.H{
			"duration": 0,
			"results": []gin.H{
				{
					"ID":           testutils.StreamFPVNotLive.ID,
					"name":         testutils.StreamFPVNotLive.Name,
					"friendlyTime": testutils.StreamFPVNotLive.FriendlyTime()},
			},
		}

		testCases := testutils.TestCases{
			"missing query": {
				Method:       http.MethodGet,
				Url:          baseUrl,
				ExpectedCode: http.StatusBadRequest,
			},
			"missing courseId": {
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("%s?q=abc", baseUrl),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid courseId": {
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("%s?q=%s&courseId=abc", baseUrl, queryString),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not perform search": {
				Method: http.MethodGet,
				Url:    fmt.Sprintf("%s?q=%s&courseId=%d", baseUrl, queryString, testutils.CourseFPV.ID),
				DaoWrapper: dao.DaoWrapper{
					SearchDao: func() dao.SearchDao {
						searchMock := mock_dao.NewMockSearchDao(ctrl)
						searchMock.
							EXPECT().
							Search(queryString, testutils.CourseFPV.ID).
							Return([]model.Stream{}, errors.New(""))
						return searchMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method: http.MethodGet,
				Url:    fmt.Sprintf("%s?q=%s&courseId=%d", baseUrl, queryString, testutils.CourseFPV.ID),
				DaoWrapper: dao.DaoWrapper{
					SearchDao: func() dao.SearchDao {
						searchMock := mock_dao.NewMockSearchDao(ctrl)
						searchMock.
							EXPECT().
							Search(queryString, testutils.CourseFPV.ID).
							Return([]model.Stream{testutils.StreamFPVNotLive}, nil)
						return searchMock
					}(),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
			},
		}

		testCases.Run(t, configGinSearchRouter)
	})
}
