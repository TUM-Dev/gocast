package api

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/matthiasreumann/gomino"
	"go.uber.org/mock/gomock"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
)

func LectureHallRouterWrapper(t *testing.T) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		configGinLectureHallApiRouter(r, dao.DaoWrapper{})
	}
}

func TestLectureHallIcal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/schedule.ics", func(t *testing.T) {
		url := "/api/schedule.ics?lecturehalls=1,2"
		calendarResultsAdmin := []dao.CalendarResult{
			{
				StreamID:        1,
				Created:         time.Now(),
				Start:           time.Now(),
				End:             time.Now(),
				CourseName:      "FPV",
				LectureHallName: "HS1",
			},

			{
				StreamID:        2,
				Created:         time.Now(),
				Start:           time.Now(),
				End:             time.Now(),
				CourseName:      "GBS",
				LectureHallName: "HS2",
			},
		}
		calendarResultsLoggedIn := []dao.CalendarResult{
			{
				StreamID:        1,
				Created:         time.Now(),
				Start:           time.Now(),
				End:             time.Now(),
				CourseName:      "FPV",
				LectureHallName: "HS1",
			},
		}
		var icalAdmin bytes.Buffer
		var icalLoggedIn bytes.Buffer
		templ, _ := template.ParseFS(staticFS, "template/*.gotemplate")
		_ = templ.ExecuteTemplate(&icalAdmin, "ical.gotemplate", calendarResultsAdmin)
		_ = templ.ExecuteTemplate(&icalLoggedIn, "ical.gotemplate", calendarResultsLoggedIn)

		gomino.TestCases{
			"no context": {
				Router:       LectureHallRouterWrapper(t),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not get streams for lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetStreamsForLectureHallIcal(gomock.Any(), []uint{1, 2}, false).
								Return(nil, errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
						AuditDao: func() dao.AuditDao {
							auditDao := mock_dao.NewMockAuditDao(gomock.NewController(t))
							auditDao.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
							return auditDao
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetStreamsForLectureHallIcal(testutils.TUMLiveContextAdmin.User.ID, []uint{1, 2}, false).
								Return(calendarResultsAdmin, nil).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedResponse: icalAdmin.Bytes(),
				ExpectedCode:     http.StatusOK,
			},
			"success student": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetStreamsForLectureHallIcal(testutils.TUMLiveContextStudent.User.ID, []uint{1, 2}, false).
								Return(calendarResultsLoggedIn, nil).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedResponse: icalLoggedIn.Bytes(),
				ExpectedCode:     http.StatusOK,
			},
		}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestLectureHallSetLH(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/setLectureHall", func(t *testing.T) {
		url := "/api/setLectureHall"
		lectureHall := testutils.LectureHall
		fpvStream := testutils.StreamFPVLive
		request := setLectureHallRequest{
			StreamIDs:     []uint{fpvStream.ID},
			LectureHallID: lectureHall.ID,
		}
		unsetLectureHallRequest := setLectureHallRequest{
			StreamIDs:     []uint{fpvStream.ID},
			LectureHallID: 0,
		}
		gomino.TestCases{
			"invalid body": {
				Router:       LectureHallRouterWrapper(t),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not get stream by id": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{}, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not unset lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{fpvStream}, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UnsetLectureHall(request.StreamIDs).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         unsetLectureHallRequest,
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not find lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByID(lectureHall.ID).
								Return(model.LectureHall{}, errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{fpvStream}, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UnsetLectureHall(request.StreamIDs).
								Return(nil).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusNotFound,
			},
			"can not set lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: testutils.GetLectureHallMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{fpvStream}, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UnsetLectureHall(request.StreamIDs).
								Return(nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								SetLectureHall(request.StreamIDs, request.LectureHallID).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: testutils.GetLectureHallMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{fpvStream}, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UnsetLectureHall(request.StreamIDs).
								Return(nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								SetLectureHall(request.StreamIDs, request.LectureHallID).
								Return(nil).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusOK,
			},
		}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}
