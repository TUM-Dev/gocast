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

func SearchRouterWrapper(r *gin.Engine) {
	configGinSearchRouter(r, dao.DaoWrapper{})
}

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

		gomino.TestCases{
			"missing query": {
				Router:       SearchRouterWrapper,
				Method:       http.MethodGet,
				Url:          baseUrl,
				ExpectedCode: http.StatusBadRequest,
			},
			"missing courseId": {
				Router:       SearchRouterWrapper,
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("%s?q=abc", baseUrl),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid courseId": {
				Router:       SearchRouterWrapper,
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("%s?q=%s&courseId=abc", baseUrl, queryString),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not perform search": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						SearchDao: func() dao.SearchDao {
							searchMock := mock_dao.NewMockSearchDao(ctrl)
							searchMock.
								EXPECT().
								Search(queryString, testutils.CourseFPV.ID).
								Return([]model.Stream{}, errors.New(""))
							return searchMock
						}(),
					}
					configGinSearchRouter(r, wrapper)
				},
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("%s?q=%s&courseId=%d", baseUrl, queryString, testutils.CourseFPV.ID),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						SearchDao: func() dao.SearchDao {
							searchMock := mock_dao.NewMockSearchDao(ctrl)
							searchMock.
								EXPECT().
								Search(queryString, testutils.CourseFPV.ID).
								Return([]model.Stream{testutils.StreamFPVNotLive}, nil)
							return searchMock
						}(),
					}
					configGinSearchRouter(r, wrapper)
				},
				Method:           http.MethodGet,
				Url:              fmt.Sprintf("%s?q=%s&courseId=%d", baseUrl, queryString, testutils.CourseFPV.ID),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}.Run(t, testutils.Equal)
	})
}
