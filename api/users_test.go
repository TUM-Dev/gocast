package api

import (
	"bytes"
	"database/sql"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"gorm.io/gorm"
	"net/http"
	"testing"
)

func UsersRouterWrapper(r *gin.Engine) {
	configGinUsersRouter(r, dao.DaoWrapper{})
}

func TestUsersCRUD(t *testing.T) {
	t.Run("/searchUser", func(t *testing.T) {
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
		response := []userSearchDTO{
			{ID: users[0].ID,
				LrzID: tools.MaskLogin(users[0].LrzID),
				Email: testutils.First(tools.MaskEmail(users[0].Email.String)).(string),
				Name:  users[0].Name,
				Role:  users[0].Role},
			{ID: users[1].ID,
				LrzID: tools.MaskLogin(users[1].LrzID),
				Email: testutils.First(tools.MaskEmail(users[1].Email.String)).(string),
				Name:  users[1].Name, Role: users[1].Role}}
		gomino.TestCases{
			"GET[Query to short]": {
				Router:       UsersRouterWrapper,
				Method:       http.MethodGet,
				Url:          "/api/searchUser?q=a",
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode: http.StatusBadRequest,
			},
			"GET[SearchUser returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().SearchUser("han").Return([]model.User{}, errors.New(""))
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodGet,
				Url:          "/api/searchUser?q=han",
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode: http.StatusInternalServerError,
			},
			"GET[success]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().SearchUser("han").Return(users, nil)
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:           http.MethodGet,
				Url:              "/api/searchUser?q=han",
				Middlewares:      testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}.Run(t, testutils.Equal)
	})

	t.Run("/createUser", func(t *testing.T) {
		url := "/api/createUser"
		userLecturer := model.User{
			Name:  "Hansi",
			Email: sql.NullString{String: "hansi@tum.de", Valid: true},
			Role:  model.LecturerType}
		request := createUserRequest{
			Name:     userLecturer.Name,
			Email:    userLecturer.Email.String,
			Password: "hansi123",
		}

		response := createUserResponse{
			Name:  userLecturer.Name,
			Email: userLecturer.Email.String,
			Role:  model.AdminType, // can only test with admin, since Mails aren't mocked yet
		}

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
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[Invalid Body]": {
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
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         bytes.NewBuffer([]byte{}),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[CreateUser(lecturer) returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil)
							usersMock.EXPECT().CreateUser(gomock.Any(), &userLecturer).Return(errors.New(""))
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[CreateUser(admin) returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(true, nil)
							usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(errors.New(""))
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
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
				Middlewares:      testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:             request,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}.Run(t, testutils.Equal)
	})

	t.Run("/users/update", func(t *testing.T) {
		url := "/api/users/update"
		userId := uint(1)
		request := struct {
			ID   uint `json:"id"`
			Role uint `json:"role"`
		}{ID: userId, Role: model.AdminType}

		gomino.TestCases{
			"POST[Invalid Body]": {
				Router:       UsersRouterWrapper,
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[GetUserByID returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().GetUserByID(gomock.Any(), userId).Return(model.User{}, errors.New("")).AnyTimes()
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[UpdateUser returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().GetUserByID(gomock.Any(), userId).Return(model.User{}, nil).AnyTimes()
							usersMock.EXPECT().UpdateUser(model.User{Role: model.AdminType}).Return(errors.New("")).AnyTimes()
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[success]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().GetUserByID(gomock.Any(), userId).Return(model.User{}, nil).AnyTimes()
							usersMock.EXPECT().UpdateUser(model.User{Role: model.AdminType}).Return(nil).AnyTimes()
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         request,
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, testutils.Equal)
	})

	t.Run("/deleteUser", func(t *testing.T) {
		url := "/api/deleteUser"
		userId := uint(1)
		request := deleteUserRequest{Id: userId}

		gomino.TestCases{
			"POST[Invalid Body]": {
				Router:       UsersRouterWrapper,
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[IsUserAdmin returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().IsUserAdmin(gomock.Any(), userId).Return(true, errors.New("")).AnyTimes()
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[IsAdmin]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().IsUserAdmin(gomock.Any(), userId).Return(true, nil).AnyTimes()
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         request,
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[DeleteUser returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().IsUserAdmin(gomock.Any(), userId).Return(false, nil).AnyTimes()
							usersMock.EXPECT().DeleteUser(gomock.Any(), userId).Return(errors.New("")).AnyTimes()
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[success]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().IsUserAdmin(gomock.Any(), userId).Return(false, nil).AnyTimes()
							usersMock.EXPECT().DeleteUser(gomock.Any(), userId).Return(nil).AnyTimes()
							return usersMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				Body:         request,
				ExpectedCode: http.StatusOK,
			},
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
			}}
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
			}}
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
				Middlewares:      testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}.Run(t, testutils.Equal)
	})
}
