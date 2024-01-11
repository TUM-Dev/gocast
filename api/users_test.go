package api

import (
	"bytes"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/matthiasreumann/gomino"
	"gorm.io/gorm"
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
			{
				ID:    users[0].ID,
				LrzID: tools.MaskLogin(users[0].LrzID),
				Email: gomino.First(tools.MaskEmail(users[0].Email.String)).(string),
				Name:  users[0].Name,
				Role:  users[0].Role,
			},
			{
				ID:    users[1].ID,
				LrzID: tools.MaskLogin(users[1].LrzID),
				Email: gomino.First(tools.MaskEmail(users[1].Email.String)).(string),
				Name:  users[1].Name, Role: users[1].Role,
			},
		}
		gomino.TestCases{
			"GET[Query to short]": {
				Router:       UsersRouterWrapper,
				Method:       http.MethodGet,
				Url:          "/api/searchUser?q=a",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
			Role:  model.LecturerType,
		}
		request := createUserRequest{
			Name:     userLecturer.Name,
			Email:    userLecturer.Email.String,
			Password: "hansi123",
		}

		response := createUserResponse{
			Name:  userLecturer.Name,
			Email: userLecturer.Email.String,
			Role:  model.LecturerType, // can only test with admin, since Mails aren't mocked yet
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[Users empty]": {
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
							usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil)
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
			"POST[getCreateUserHandlers(lecturer) returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil)
							usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(errors.New(""))
							usersMock.EXPECT().GetUserByEmail(gomock.Any(), request.Email).Return(testutils.Lecturer, nil).AnyTimes()
							usersMock.EXPECT().CreateRegisterLink(gomock.Any(), testutils.Lecturer).Return(model.RegisterLink{}, nil).AnyTimes()
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
							usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil)
							usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil)
							usersMock.EXPECT().GetUserByEmail(gomock.Any(), request.Email).Return(testutils.Lecturer, nil).AnyTimes()
							usersMock.EXPECT().CreateRegisterLink(gomock.Any(), testutils.Lecturer).Return(model.RegisterLink{}, nil).AnyTimes()
							return usersMock
						}(),
						EmailDao: func() dao.EmailDao {
							emailMock := mock_dao.NewMockEmailDao(gomock.NewController(t))
							emailMock.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).MinTimes(1).MaxTimes(1)
							return emailMock
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
			},
		}.Run(t, testutils.Equal)
	})

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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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

func TestResetPassword(t *testing.T) {
	t.Run("/api/users/resetPassword", func(t *testing.T) {
		hansi := model.User{
			Model: gorm.Model{ID: 1},
			Name:  "Hansi",
			Email: sql.NullString{String: "hansi@tum.de", Valid: true},
			Role:  model.StudentType,
		}
		ctrl := gomock.NewController(t)
		tools.Cfg.Mail = tools.MailConfig{Sender: "from@invalid", Server: "server", SMIMECert: "", SMIMEKey: "", MaxMailsPerMinute: 1}
		gomino.TestCases{
			"POST[success]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(ctrl)
							usersMock.EXPECT().CreateRegisterLink(gomock.Any(), hansi).Return(model.RegisterLink{RegisterSecret: "abc"}, nil).MinTimes(1).MaxTimes(1)

							usersMock.EXPECT().GetUserByEmail(gomock.Any(), hansi.Email.String).Return(hansi, nil).MinTimes(1).MaxTimes(1)
							return usersMock
						}(),
						EmailDao: func() dao.EmailDao {
							emailMock := mock_dao.NewMockEmailDao(ctrl)
							emailMock.EXPECT().Create(gomock.Any(), &model.Email{
								From:    tools.Cfg.Mail.Sender,
								To:      hansi.Email.String,
								Subject: "TUM-Live: Reset Password",
								Body:    "Hi! \n\nYou can reset your TUM-Live password by clicking on the following link: \n\n" + tools.Cfg.WebUrl + "/setPassword/abc\n\nIf you did not request a password reset, please ignore this email. \n\nBest regards",
							}).Return(nil).MinTimes(1).MaxTimes(1)
							return emailMock
						}(),
					}
					configGinUsersRouter(r, wrapper)
				},
				Method: http.MethodPost,
				Url:    "/api/users/resetPassword",
				Body: struct {
					Username string `json:"username"`
				}{Username: "hansi@tum.de"},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: nil,
			},
		}.Run(t, testutils.Equal)
	})
}
