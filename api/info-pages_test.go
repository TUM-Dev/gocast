package api

import (
	"errors"
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"html/template"
	"net/http"
	"testing"
)

func InfoPagesRouterWrapper(r *gin.Engine) {
	configInfoPageRouter(r, dao.DaoWrapper{})
}

func TestInfoPagesCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	templateExecutor := tools.ReleaseTemplateExecutor{
		Template: template.Must(template.New("base").Funcs(sprig.FuncMap()).
			ParseFiles("../web/template/error.gohtml")),
	}
	tools.SetTemplateExecutor(templateExecutor)

	req := updateTextDao{
		Name:       "Data Privacy",
		RawContent: "#Data privacy",
		Type:       model.INFOPAGE_MARKDOWN,
	}

	url := fmt.Sprintf("/api/texts/%d", testutils.InfoPage.ID)
	t.Run("PUT/api/texts/:id", func(t *testing.T) {
		gomino.TestCases{
			"no context": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid id": {
				Url:          "/api/texts/abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         req,
				ExpectedCode: http.StatusBadRequest,
			},
			"Update returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						InfoPageDao: func() dao.InfoPageDao {
							infoPageMock := mock_dao.NewMockInfoPageDao(gomock.NewController(t))
							infoPageMock.
								EXPECT().
								Update(testutils.InfoPage.ID, &model.InfoPage{
									Name:       req.Name,
									RawContent: req.RawContent,
									Type:       req.Type,
								}).
								Return(errors.New("")).
								AnyTimes()
							return infoPageMock
						}(),
					}
					configInfoPageRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         req,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						InfoPageDao: func() dao.InfoPageDao {
							infoPageMock := mock_dao.NewMockInfoPageDao(gomock.NewController(t))
							infoPageMock.
								EXPECT().
								Update(testutils.InfoPage.ID, &model.InfoPage{
									Name:       req.Name,
									RawContent: req.RawContent,
									Type:       req.Type,
								}).
								Return(nil).
								AnyTimes()
							return infoPageMock
						}(),
					}
					configInfoPageRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         req,
				ExpectedCode: http.StatusOK,
			}}.
			Router(InfoPagesRouterWrapper).
			Method(http.MethodPut).
			Url(url).
			Run(t, testutils.Equal)
	})
}
