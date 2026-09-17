package api

import (
	"bytes"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/matthiasreumann/gomino"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
)

func UsersRouterWrapper(r *gin.Engine) {
	configGinUsersRouter(r, dao.DaoWrapper{})
}

func TestUsersCRUD(t *testing.T) {
	t.Run("/users/init", func(t *testing.T) {
		url := "/api/users/init"
		/*initialUser := model.User{
			Name:  "Hansi",
			Email: sql.NullString{String: "hansi@tum.de", Valid: true},
			Role:  model.AdminType}
		request := createUserRequest{
			Name:     initialUser.Name,
			Email:    initialUser.Email.String,
			Password: "hansi123",
		}

		response := createUserResponse{
			Name:  initialUser.Name,
			Email: initialUser.Email.String,
			Role:  model.AdminType, // can only test with admin, since Mails aren't mocked yet
		}*/

		gomino.TestCases{
			"POST[AreUsersEmpty returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, errors.New(""))
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[Users not empty]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil)
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[Invalid Body]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(true, nil)
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         bytes.NewBuffer([]byte{}),
				ExpectedCode: http.StatusBadRequest,
			},
			/*
				FAILS BECAUSE OF CERTIFICATE CHECK
				"POST[getCreateUserHandlers(admin) returns error]": {
					Router: func(r *gin.Engine) {
						wrapper := dao.DaoWrapper{
							UsersDao: func() dao.UsersDao {
								usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
								usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(true, nil)
								usersMock.EXPECT().CreateUser(gomock.Any(), &initialUser).Return(errors.New(""))
								return usersMock
							}(),
						}
						configGinUsersRouter(r, wrapper)
					},
					Method:       http.MethodPost,
					Url:          url,
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
					Body:         request,
					ExpectedCode: http.StatusInternalServerError,
				},
				"POST[success]": {
					Router: func(r *gin.Engine) {
						wrapper := dao.DaoWrapper{
							UsersDao: func() dao.UsersDao {
								usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
								usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(true, nil)
								usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil)
								return usersMock
							}(),
						}
						configGinUsersRouter(r, wrapper)
					},
					Method:           http.MethodPost,
					Url:              url,
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
					Body:             request,
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: response,
				},*/
		}.Run(t, testutils.Equal)
	})
}

func TestSearchUserForCourse(t *testing.T) {
	t.Run("/searchUserForCourse", func(t *testing.T) {
		users := []model.User{
			{
				Model: gorm.Model{ID: 1},
				Name:  "Hansi",
				Email: sql.NullString{String: "hansi@tum.de", Valid: true},
				Role:  model.StudentType,
			},
			{
				Model: gorm.Model{ID: 2},
				Name:  "Hannes",
				Email: sql.NullString{String: "hannes@tum.de", Valid: true},
				Role:  model.StudentType,
			},
		}
		response := []userForLecturerDto{
			{
				ID:       users[0].ID,
				Name:     users[0].Name,
				LastName: users[0].LastName,
				Login:    users[0].GetLoginString(),
			},
			{
				ID:       users[1].ID,
				Name:     users[1].Name,
				LastName: users[1].LastName,
				Login:    users[1].GetLoginString(),
			},
		}
		gomino.TestCases{
			"GET[success]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().SearchUser("han").Return(users, nil).AnyTimes()
							return usersMock
						}(),
					}

					configGinUsersRouter(r, wrapper)
				},
				Method:           http.MethodGet,
				Url:              "/api/searchUserForCourse?q=han",
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}.Run(t, testutils.Equal)
	})
}
