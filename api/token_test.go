package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
	"time"
)

func TestToken(t *testing.T) {
	t.Run("/create", func(t *testing.T) {
		now := time.Now()
		type req struct {
			Expires *time.Time `json:"expires"`
			Scope   string     `json:"scope"`
		}
		testutils.TestCases{
			"POST[No Context]": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            "/api/token/create",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},

			"POST[Invalid Body]": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            "/api/token/create",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{Role: model.AdminType}},
				Body:           bytes.NewBuffer([]byte{}),
				ExpectedCode:   http.StatusBadRequest,
			},
			"POST[Invalid Scope]": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            "/api/token/create",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{Role: model.AdminType}},
				Body: bytes.NewBuffer(testutils.First(json.Marshal(req{
					Expires: &now,
					Scope:   "invalid",
				})).([]byte)),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[AddToken returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/token/create",
				DaoWrapper: dao.DaoWrapper{
					TokenDao: func() dao.TokenDao {
						tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
						tokenMock.EXPECT().AddToken(gomock.Any()).Return(errors.New("")).AnyTimes()
						return tokenMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{Role: model.AdminType}},
				Body: bytes.NewBuffer(testutils.First(json.Marshal(req{
					Expires: &now,
					Scope:   model.TokenScopeAdmin,
				})).([]byte)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[success]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/token/create",
				DaoWrapper: dao.DaoWrapper{
					TokenDao: func() dao.TokenDao {
						tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
						tokenMock.EXPECT().AddToken(gomock.Any()).Return(nil).AnyTimes()
						return tokenMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{Role: model.AdminType}},
				Body: bytes.NewBuffer(testutils.First(json.Marshal(req{
					Expires: &now,
					Scope:   model.TokenScopeAdmin,
				})).([]byte)),
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, configTokenRouter)
	})

	t.Run("/:id", func(t *testing.T) {
		testutils.TestCases{
			"DELETE[DeleteToken returns error]": testutils.TestCase{
				Method: http.MethodDelete,
				Url:    "/api/token/1",
				DaoWrapper: dao.DaoWrapper{
					TokenDao: func() dao.TokenDao {
						tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
						tokenMock.EXPECT().DeleteToken("1").Return(errors.New("")).AnyTimes()
						return tokenMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{Role: model.AdminType}},
				ExpectedCode:   http.StatusInternalServerError,
			},
			"DELETE[Success]": testutils.TestCase{
				Method: http.MethodDelete,
				Url:    "/api/token/1",
				DaoWrapper: dao.DaoWrapper{
					TokenDao: func() dao.TokenDao {
						tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
						tokenMock.EXPECT().DeleteToken("1").Return(nil).AnyTimes()
						return tokenMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{Role: model.AdminType}},
				ExpectedCode:   http.StatusOK,
			},
		}.Run(t, configTokenRouter)
	})
}
