package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"html/template"
	"net/http"
	"testing"
)

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
	body := testutils.First(json.Marshal(req)).([]byte)

	url := fmt.Sprintf("/api/texts/%d", testutils.InfoPage.ID)
	t.Run("PUT/api/texts/:id", func(t *testing.T) {
		testutils.TestCases{
			"no context": {
				Method:         http.MethodPut,
				Url:            url,
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPut,
				Url:            url,
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"invalid body": {
				Method:         http.MethodPut,
				Url:            url,
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid id": {
				Method:         http.MethodPut,
				Url:            "/api/texts/abc",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(body),
				ExpectedCode:   http.StatusBadRequest,
			},
			"Update returns error": {
				Method: http.MethodPut,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
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
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(body),
				ExpectedCode:   http.StatusBadRequest,
			},
			"success": {
				Method: http.MethodPut,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
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
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(body),
				ExpectedCode:   http.StatusOK,
			},
		}.Run(t, configInfoPageRouter)
	})
}
