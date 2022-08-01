package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
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
				Url:          baseUrl,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusBadRequest,
			},
			"missing courseId": {
				Url:          fmt.Sprintf("%s?q=abc", baseUrl),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid courseId": {
				Url:          fmt.Sprintf("%s?q=%s&courseId=abc", baseUrl, queryString),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not perform search": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						SearchDao: func() dao.SearchDao {
							searchMock := mock_dao.NewMockSearchDao(ctrl)
							searchMock.
								EXPECT().
								Search(gomock.Any(), queryString, testutils.CourseFPV.ID).
								Return([]model.Stream{}, errors.New(""))
							return searchMock
						}(),
					}
					configGinSearchRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						SearchDao: func() dao.SearchDao {
							searchMock := mock_dao.NewMockSearchDao(ctrl)
							searchMock.
								EXPECT().
								Search(gomock.Any(), queryString, testutils.CourseFPV.ID).
								Return([]model.Stream{testutils.StreamFPVNotLive}, nil)
							return searchMock
						}(),
					}
					configGinSearchRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			}}.
			Router(SearchRouterWrapper).
			Method(http.MethodGet).
			Url(fmt.Sprintf("%s?q=%s&courseId=%d", baseUrl, queryString, testutils.CourseFPV.ID)).
			Run(t, testutils.Equal)
	})
}
