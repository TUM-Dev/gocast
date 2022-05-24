package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"gorm.io/gorm"
	"net/http"
	"testing"
)

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
		response := testutils.First(json.Marshal([]userSearchDTO{
			{ID: users[0].ID,
				LrzID: tools.MaskLogin(users[0].LrzID),
				Email: testutils.First(tools.MaskEmail(users[0].Email.String)).(string),
				Name:  users[0].Name,
				Role:  users[0].Role},
			{ID: users[1].ID,
				LrzID: tools.MaskLogin(users[1].LrzID),
				Email: testutils.First(tools.MaskEmail(users[1].Email.String)).(string),
				Name:  users[1].Name, Role: users[1].Role},
		})).([]byte)
		testCases := testutils.TestCases{
			"GET[Query to short]": testutils.TestCase{
				Method:     http.MethodGet,
				Url:        "/api/searchUser?q=a",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				ExpectedCode: http.StatusBadRequest,
			},
			"GET[SearchUser returns error]": testutils.TestCase{
				Method: http.MethodGet,
				Url:    "/api/searchUser?q=han",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().SearchUser("han").Return([]model.User{}, errors.New(""))
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				ExpectedCode: http.StatusInternalServerError,
			},
			"GET[success]": testutils.TestCase{
				Method: http.MethodGet,
				Url:    "/api/searchUser?q=han",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().SearchUser("han").Return(users, nil)
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}

		testCases.Run(t, configGinUsersRouter)
	})

	t.Run("/createUser", func(t *testing.T) {
		userLecturer := model.User{
			Name:  "Hansi",
			Email: sql.NullString{String: "hansi@tum.de", Valid: true},
			Role:  model.LecturerType}
		request := testutils.First(json.Marshal(createUserRequest{
			Name:     userLecturer.Name,
			Email:    userLecturer.Email.String,
			Password: "hansi123",
		})).([]byte)

		response := testutils.First(json.Marshal(createUserResponse{
			Name:  userLecturer.Name,
			Email: userLecturer.Email.String,
			Role:  model.AdminType, // can only test with admin, since Mails aren't mocked yet
		})).([]byte)

		testCases := testutils.TestCases{
			"POST[AreUsersEmpty returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/createUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, errors.New(""))
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[Invalid Body]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/createUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil)
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer([]byte{}),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[CreateUser(lecturer) returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/createUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil)
						usersMock.EXPECT().CreateUser(gomock.Any(), &userLecturer).Return(errors.New(""))
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[CreateUser(admin) returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/createUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(true, nil)
						usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(errors.New(""))
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[success]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/createUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(true, nil)
						usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil)
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:             bytes.NewBuffer(request),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}

		testCases.Run(t, configGinUsersRouter)
	})

	t.Run("/users/update", func(t *testing.T) {
		userId := uint(1)
		request := testutils.First(json.Marshal(struct {
			ID   uint `json:"id"`
			Role uint `json:"role"`
		}{
			ID:   userId,
			Role: model.AdminType,
		})).([]byte)

		testCases := testutils.TestCases{
			"POST[Invalid Body]": testutils.TestCase{
				Method:     http.MethodPost,
				Url:        "/api/users/update",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer([]byte{}),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[GetUserByID returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/users/update",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().GetUserByID(gomock.Any(), userId).Return(model.User{}, errors.New("")).AnyTimes()
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[UpdateUser returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/users/update",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().GetUserByID(gomock.Any(), userId).Return(model.User{}, nil).AnyTimes()
						usersMock.EXPECT().UpdateUser(model.User{Role: model.AdminType}).Return(errors.New("")).AnyTimes()
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[success]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/users/update",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().GetUserByID(gomock.Any(), userId).Return(model.User{}, nil).AnyTimes()
						usersMock.EXPECT().UpdateUser(model.User{Role: model.AdminType}).Return(nil).AnyTimes()
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configGinUsersRouter)
	})

	t.Run("/deleteUser", func(t *testing.T) {
		userId := uint(1)
		request := testutils.First(json.Marshal(deleteUserRequest{
			Id: userId,
		})).([]byte)

		testCases := testutils.TestCases{
			"POST[Invalid Body]": testutils.TestCase{
				Method:     http.MethodPost,
				Url:        "/api/deleteUser",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer([]byte{}),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[IsUserAdmin returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/deleteUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().IsUserAdmin(gomock.Any(), userId).Return(true, errors.New("")).AnyTimes()
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[IsAdmin]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/deleteUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().IsUserAdmin(gomock.Any(), userId).Return(true, nil).AnyTimes()
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[DeleteUser returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/deleteUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().IsUserAdmin(gomock.Any(), userId).Return(false, nil).AnyTimes()
						usersMock.EXPECT().DeleteUser(gomock.Any(), userId).Return(errors.New("")).AnyTimes()
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST[success]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/deleteUser",
				DaoWrapper: dao.DaoWrapper{
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.EXPECT().IsUserAdmin(gomock.Any(), userId).Return(false, nil).AnyTimes()
						usersMock.EXPECT().DeleteUser(gomock.Any(), userId).Return(nil).AnyTimes()
						return usersMock
					}(),
				},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         bytes.NewBuffer(request),
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configGinUsersRouter)
	})
}
