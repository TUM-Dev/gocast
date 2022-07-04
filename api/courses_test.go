package api

import (
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
	"github.com/u2takey/go-utils/uuid"
	"gorm.io/gorm"
	"html/template"
	"net/http"
	"testing"
	"time"
)

func TestCoursesCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE/api/course/:courseID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/", testutils.CourseFPV.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusForbidden,
			},
			/*
				TODO: Mock Cache object
				"success": {
					Method: http.MethodDelete,
					Url:    url,
					DaoWrapper: dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								DeleteCourse(gomock.Any())
							return coursesMock
						}(),
					},
					TumLiveContext: &testutils.TUMLiveContextAdmin,
					ExpectedCode:   http.StatusOK,
				},
			*/
		}

		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("POST/api/createCourse", func(t *testing.T) {
		url := "/api/createCourse"

		templateExecutor := tools.ReleaseTemplateExecutor{
			Template: template.Must(template.New("base").Funcs(sprig.FuncMap()).
				ParseFiles("../web/template/error.gohtml")),
		}
		tools.SetTemplateExecutor(templateExecutor)

		request := createCourseRequest{
			Access:       "enrolled",
			EnChat:       false,
			EnDL:         false,
			EnVOD:        false,
			Name:         "New Course",
			Slug:         "NC",
			TeachingTerm: "Sommersemester 2020",
		}

		requestInvalidAccess := createCourseRequest{
			Access:       "abc",
			EnChat:       false,
			EnDL:         false,
			EnVOD:        false,
			Name:         "New Course",
			Slug:         "NC",
			TeachingTerm: "Sommersemester 2020",
		}

		requestInvalidTerm := createCourseRequest{
			Access:       "enrolled",
			EnChat:       false,
			EnDL:         false,
			EnVOD:        false,
			Name:         "New Course",
			Slug:         "NC",
			TeachingTerm: "Sommersemester 20",
		}

		newCourse := model.Course{
			UserID:              testutils.Lecturer.ID,
			Name:                request.Name,
			Slug:                request.Slug,
			Year:                2020, // Taken from 'request'
			TeachingTerm:        "S",  // Taken from 'request'
			TUMOnlineIdentifier: request.CourseID,
			VODEnabled:          request.EnVOD,
			DownloadsEnabled:    request.EnDL,
			ChatEnabled:         request.EnChat,
			Visibility:          request.Access,
			Streams:             []model.Stream{},
			Admins:              []model.User{testutils.Lecturer},
		}

		ctrl := gomock.NewController(t)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not lecturer": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextLecturer,
				Body:           nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid access": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextLecturer,
				Body:           requestInvalidAccess,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid term": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextLecturer,
				Body:           requestInvalidTerm,
				ExpectedCode:   http.StatusBadRequest,
			},
			"conflict with existing course": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextLecturer,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(ctrl)
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
							Return(model.Course{}, nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusConflict,
			},
			"can not create course": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextLecturer,
				DaoWrapper: dao.DaoWrapper{
					AuditDao: testutils.GetAuditMock(t),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(ctrl)
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
							Return(model.Course{}, errors.New("")).
							AnyTimes()
						coursesMock.
							EXPECT().
							CreateCourse(gomock.Any(), &newCourse, true).
							Return(errors.New("")).
							AnyTimes()
						return coursesMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not get new course": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextLecturer,
				DaoWrapper: dao.DaoWrapper{
					AuditDao: testutils.GetAuditMock(t),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(ctrl)
						first := coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
							Return(model.Course{}, errors.New("")).Times(1)
						second := coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
							Return(newCourse, errors.New("")).Times(1)

						gomock.InOrder(first, second)

						coursesMock.
							EXPECT().
							CreateCourse(gomock.Any(), &newCourse, true).
							Return(nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			/*
				TODO: Mock tum package functions
				"success S": {
					Method:         http.MethodPost,
					Url:            url,
					TumLiveContext: &testutils.TUMLiveContextLecturer,
					DaoWrapper: dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							first := coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
								Return(model.Course{}, errors.New("")).Times(1)
							second := coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
								Return(newCourse, nil).Times(1)

							gomock.InOrder(first, second)

							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), &newCourse, true).
								Return(nil).
								AnyTimes()
							return coursesMock
						}(),
					},
					Body: bytes.NewBuffer(
						testutils.First(json.Marshal(request)).([]byte)),
					ExpectedCode: http.StatusOK,
				},
			*/
		}
		testCases.Run(t, configGinCourseRouter)
	})
}

func TestCoursesLectureActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/createLecture", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/createLecture", testutils.CourseFPV.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"lectureHallId set on 'premiere'": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body: createLectureRequest{
					LectureHallId: "1",
					Premiere:      true,
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid lectureHallId": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body: createLectureRequest{
					Title:         "Lecture 1",
					LectureHallId: "abc",
					Start:         time.Now(),
					Duration:      90,
					Premiere:      false,
					Vodup:         false,
					DateSeries:    []time.Time{},
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"can not update course": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							UpdateCourse(gomock.Any(), gomock.Any()).
							Return(errors.New(""))
						return coursesMock
					}(),
					AuditDao: func() dao.AuditDao {
						auditMock := mock_dao.NewMockAuditDao(gomock.NewController(t))
						auditMock.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
						return auditMock
					}(),
				},
				Body: createLectureRequest{
					Title:         "Lecture 1",
					LectureHallId: "1",
					Start:         time.Now(),
					Duration:      90,
					Premiere:      false,
					Vodup:         false,
					DateSeries: []time.Time{
						time.Now(),
					},
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							UpdateCourse(gomock.Any(), gomock.Any()).
							Return(nil)
						return coursesMock
					}(),
					AuditDao: func() dao.AuditDao {
						auditMock := mock_dao.NewMockAuditDao(gomock.NewController(t))
						auditMock.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
						return auditMock
					}(),
				},
				Body: createLectureRequest{
					Title:         "Lecture 1",
					LectureHallId: "1",
					Start:         time.Now(),
					Duration:      90,
					Premiere:      false,
					Vodup:         false,
					DateSeries: []time.Time{
						time.Now(),
					},
				},
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("POST/api/course/:courseID/deleteLecture", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/deleteLectures", testutils.CourseFPV.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid stream id in body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamGBSLive.ID)).
							Return(testutils.StreamGBSLive, nil).
							AnyTimes()
						return streamsMock
					}(),
				},
				Body: deleteLecturesRequest{StreamIDs: []string{
					fmt.Sprintf("%d", testutils.StreamGBSLive.ID)},
				},
				ExpectedCode: http.StatusForbidden,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							DeleteStream(fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return().
							AnyTimes()
						return streamsMock
					}(),
					AuditDao: testutils.GetAuditMock(t),
				},
				Body: deleteLecturesRequest{StreamIDs: []string{
					fmt.Sprintf("%d", testutils.StreamFPVLive.ID)},
				},
				ExpectedCode: http.StatusOK,
			},
		}
		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("POST/api/course/:courseID/renameLecture/:streamID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/renameLecture/%d", testutils.CourseFPV.ID, testutils.StreamFPVLive.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid streamID": {
				Method:         http.MethodPost,
				Url:            fmt.Sprintf("/api/course/%d/renameLecture/abc", testutils.CourseFPV.ID),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"stream not found": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(model.Stream{}, errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body: renameLectureRequest{
					Name: "Proofs #1",
				},
				ExpectedCode: http.StatusNotFound,
			},
			"can not update stream": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							UpdateStream(gomock.Any()).
							Return(errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body: renameLectureRequest{
					Name: "Proofs #1",
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							UpdateStream(gomock.Any()).
							Return(nil).
							AnyTimes()
						return streamsMock
					}(),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body: renameLectureRequest{
					Name: "Proofs #1",
				},
				ExpectedCode: http.StatusOK,
			},
		}
		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("POST/api/course/:courseID/updateLectureSeries/:streamID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/updateLectureSeries/%d", testutils.CourseFPV.ID, testutils.StreamFPVLive.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"stream not found": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusNotFound,
			},
			"can not update lecture series": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							UpdateLectureSeries(testutils.StreamFPVLive).
							Return(errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							UpdateLectureSeries(testutils.StreamFPVLive).
							Return(nil).
							AnyTimes()
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusOK,
			},
		}
		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("DELETE/api/course/:courseID/deleteLectureSeries/:streamID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/deleteLectureSeries/%d", testutils.CourseFPV.ID, testutils.StreamFPVLive.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"stream not found": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusNotFound,
			},
			"invalid series-identifier": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), gomock.Any()).
							Return(testutils.StreamGBSLive, nil). //StreamGBSLive.SeriesIdentifier == ""
							AnyTimes()
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"can not delete lecture-series": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					AuditDao:   testutils.GetAuditMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), gomock.Any()).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							DeleteLectureSeries(testutils.StreamFPVLive.SeriesIdentifier).
							Return(errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					AuditDao:   testutils.GetAuditMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), gomock.Any()).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							DeleteLectureSeries(testutils.StreamFPVLive.SeriesIdentifier).
							Return(nil).
							AnyTimes()
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusOK,
			},
		}
		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("PUT/api/course/:courseID/updateDescription/:streamID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/updateDescription/%d", testutils.CourseFPV.ID, testutils.StreamFPVLive.ID)

		body := renameLectureRequest{
			Name: "New lecture name!",
		}
		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid streamID": {
				Method:         http.MethodPut,
				Url:            fmt.Sprintf("/api/course/%d/updateDescription/abc", testutils.CourseFPV.ID),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid body": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find stream": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				Body:         body,
				ExpectedCode: http.StatusNotFound,
			},
			"can not update stream": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							UpdateStream(gomock.Any()).
							Return(errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				Body:         body,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: testutils.GetStreamMock(t),
				},
				Body:         body,
				ExpectedCode: http.StatusOK,
			},
		}
		testCases.Run(t, configGinCourseRouter)
	})
}

func TestUnits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/addUnit", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/addUnit", testutils.CourseFPV.ID)

		request := addUnitRequest{
			LectureID:   testutils.StreamFPVLive.ID,
			From:        0,
			To:          42,
			Title:       "New Unit",
			Description: "This is a new one!",
		}

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find stream": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusNotFound,
			},
			"can not update stream associations": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							UpdateStreamFullAssoc(gomock.Any()).
							Return(errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: testutils.GetStreamMock(t),
				},
				Body:         request,
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("POST/api/course/:courseID/deleteUnit/:unitID", func(t *testing.T) {
		unit := testutils.StreamFPVLive.Units[0]
		url := fmt.Sprintf("/api/course/%d/deleteUnit/%d",
			testutils.CourseFPV.ID, unit.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"can not find unit": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetUnitByID(fmt.Sprintf("%d", unit.ID)).
							Return(unit, errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusNotFound,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: testutils.GetStreamMock(t),
				},
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
}

func TestCuts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/submitCut", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/submitCut", testutils.CourseFPV.ID)

		request := submitCutRequest{
			LectureID: testutils.StreamFPVLive.ID,
			From:      0,
			To:        1000,
		}
		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find stream": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusNotFound,
			},
			"can not update stream": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).
							AnyTimes()
						streamsMock.
							EXPECT().
							SaveStream(gomock.Any()).
							Return(errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: testutils.GetStreamMock(t),
				},
				Body:         request,
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
}

func TestAdminFunctions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/course/:courseID/admins", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/admins", testutils.CourseFPV.ID)

		response := []userForLecturerDto{
			{
				ID:    testutils.Admin.ID,
				Name:  testutils.Admin.Name,
				Login: testutils.Admin.GetLoginString(),
			},
			{
				ID:    testutils.Admin.ID,
				Name:  testutils.Admin.Name,
				Login: testutils.Admin.GetLoginString(),
			},
		}
		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"can not get course admins": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(),
								testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm,
								testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseAdmins(testutils.CourseFPV.ID).
							Return([]model.User{}, errors.New("")).
							AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("PUT/api/course/:courseID/admins/:userID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/admins/%d", testutils.CourseFPV.ID, testutils.Admin.ID)
		urlStudent := fmt.Sprintf("/api/course/%d/admins/%d", testutils.CourseFPV.ID, testutils.Student.ID)

		resAdmin := userForLecturerDto{
			ID:    testutils.Admin.ID,
			Name:  testutils.Admin.Name,
			Login: testutils.Admin.GetLoginString(),
		}

		resStudent := userForLecturerDto{
			ID:    testutils.Student.ID,
			Name:  testutils.Student.Name,
			Login: testutils.Student.GetLoginString(),
		}

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid userID": {
				Method:         http.MethodPut,
				Url:            fmt.Sprintf("/api/course/%d/admins/abc", testutils.CourseFPV.ID),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"user not found": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.
							EXPECT().
							GetUserByID(gomock.Any(), testutils.Admin.ID).
							Return(testutils.Admin, errors.New("")).AnyTimes()
						return usersMock
					}(),
				},
				ExpectedCode: http.StatusNotFound,
			},
			"can not add admin to course": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(),
								testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							AddAdminToCourse(testutils.Admin.ID, testutils.CourseFPV.ID).
							Return(errors.New("")).
							AnyTimes()
						return coursesMock
					}(),
					UsersDao: testutils.GetUsersMock(t),
					AuditDao: testutils.GetAuditMock(t),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not update user": {
				Method:         http.MethodPut,
				Url:            urlStudent,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(),
								testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							AddAdminToCourse(testutils.Student.ID, testutils.CourseFPV.ID).
							Return(nil).
							AnyTimes()
						return coursesMock
					}(),
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.
							EXPECT().
							GetUserByID(gomock.Any(), testutils.Student.ID).
							Return(testutils.Student, nil).
							AnyTimes()
						usersMock.
							EXPECT().
							UpdateUser(gomock.Any()).
							Return(errors.New("")).
							AnyTimes()
						return usersMock
					}(),
					AuditDao: testutils.GetAuditMock(t),
				},
				ExpectedCode:     http.StatusInternalServerError,
				ExpectedResponse: testutils.First(json.Marshal(resStudent)).([]byte),
			},
			"success": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					UsersDao:   testutils.GetUsersMock(t),
					AuditDao:   testutils.GetAuditMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(resAdmin)).([]byte),
			},
			"success, user not admin": {
				Method:         http.MethodPut,
				Url:            urlStudent,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(),
								testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							AddAdminToCourse(testutils.Student.ID, testutils.CourseFPV.ID).
							Return(nil).
							AnyTimes()
						return coursesMock
					}(),
					UsersDao: func() dao.UsersDao {
						usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
						usersMock.
							EXPECT().
							GetUserByID(gomock.Any(), testutils.Student.ID).
							Return(testutils.Student, nil).
							AnyTimes()
						usersMock.
							EXPECT().
							UpdateUser(gomock.Any()).
							Return(nil).
							AnyTimes()
						return usersMock
					}(),
					AuditDao: testutils.GetAuditMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(resStudent)).([]byte),
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
	t.Run("DELETE/api/course/:courseID/admins/:userID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/admins/%d", testutils.CourseFPV.ID, testutils.Admin.ID)

		response := userForLecturerDto{
			ID:    testutils.Admin.ID,
			Name:  testutils.Admin.Name,
			Login: testutils.Admin.GetLoginString(),
		}

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid userID": {
				Method:         http.MethodDelete,
				Url:            fmt.Sprintf("/api/course/%d/admins/abc", testutils.CourseFPV.ID),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"can not get course admins": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(),
								testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseAdmins(testutils.CourseFPV.ID).
							Return([]model.User{}, errors.New("")).
							AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"remove last admin": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(),
								testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseAdmins(testutils.CourseFPV.ID).
							Return([]model.User{testutils.Admin}, nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid delete request": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(),
								testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseAdmins(testutils.CourseFPV.ID).
							Return([]model.User{testutils.Student}, nil). // student.id != admin.id from url
							AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"can not remove admin": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					AuditDao: testutils.GetAuditMock(t),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(),
								testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							GetCourseAdmins(testutils.CourseFPV.ID).
							Return([]model.User{testutils.Admin, testutils.Admin}, nil).
							AnyTimes()
						coursesMock.
							EXPECT().
							RemoveAdminFromCourse(testutils.Admin.ID, testutils.CourseFPV.ID).
							Return(errors.New("")).
							AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					AuditDao:   testutils.GetAuditMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
}

func TestLectureHallsById(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/lecture-halls-by-id", func(t *testing.T) {
		url := fmt.Sprintf("/api/lecture-halls-by-id?id=%d", testutils.CourseFPV.ID)

		ctrl := gomock.NewController(t)

		response := []lhResp{
			{
				LectureHallName: testutils.LectureHall.Name,
				Presets:         testutils.LectureHall.CameraPresets},
		}

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"invalid id": {
				Method:         http.MethodGet,
				Url:            "/api/lecture-halls-by-id?id=abc",
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusBadRequest,
			},
			"course not found": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(ctrl)
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, errors.New("")).
							AnyTimes()
						return coursesMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusNotFound,
			},
			"is not admin of course": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(ctrl)
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"success": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.
							EXPECT().
							GetLectureHallByID(testutils.LectureHall.ID).
							Return(testutils.LectureHall, nil)
						return lectureHallMock
					}(),
				},
				TumLiveContext:   &testutils.TUMLiveContextAdmin,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
}

func TestActivateToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/activate/:token", func(t *testing.T) {
		token := uuid.NewUUID()
		url := fmt.Sprintf("/api/course/activate/%s", token)

		ctrl := gomock.NewController(t)

		testCases := testutils.TestCases{
			"course not found": testutils.TestCase{
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(ctrl)
						coursesMock.
							EXPECT().
							GetCourseByToken(token).
							Return(model.Course{}, errors.New(""))
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"can not un-delete course": testutils.TestCase{
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(ctrl)
						coursesMock.
							EXPECT().
							GetCourseByToken(token).
							Return(model.Course{}, nil)
						coursesMock.
							EXPECT().
							UnDeleteCourse(gomock.Any(), gomock.Any()).
							Return(errors.New(""))
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": testutils.TestCase{
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						courseCopy := testutils.CourseFPV
						courseCopy.DeletedAt = gorm.DeletedAt{Valid: false}
						courseCopy.VODEnabled = true
						courseCopy.Visibility = "loggedin"

						coursesMock := mock_dao.NewMockCoursesDao(ctrl)
						coursesMock.
							EXPECT().
							GetCourseByToken(token).
							Return(courseCopy, nil)
						coursesMock.
							EXPECT().
							UnDeleteCourse(gomock.Any(), courseCopy).
							Return(nil)
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
}

func TestPresets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/presets", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/presets", testutils.CourseFPV.ID)

		request := []lhResp{
			{
				LectureHallName: "HS-4",
				Presets: []model.CameraPreset{
					{
						Name:          "Preset 1",
						PresetID:      1,
						Image:         "375ed239-c37d-450e-9d4f-1fbdb5a2dec5.jpg",
						LectureHallId: testutils.LectureHall.ID,
						IsDefault:     false,
					},
				},
				SelectedIndex: 1,
			},
		}

		presetSettings := []model.CameraPresetPreference{
			{
				LectureHallID: testutils.LectureHall.ID,
				PresetID:      1,
			},
		}

		afterSetPresetPreference := testutils.CourseFPV
		afterSetPresetPreference.CameraPresetPreferences = string(testutils.First(json.Marshal(presetSettings)).([]byte))

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"can not update course": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					AuditDao: testutils.GetAuditMock(t),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()

						coursesMock.
							EXPECT().
							UpdateCourse(gomock.Any(), afterSetPresetPreference).
							Return(errors.New(""))
						return coursesMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					AuditDao: testutils.GetAuditMock(t),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()

						coursesMock.
							EXPECT().
							UpdateCourse(gomock.Any(), afterSetPresetPreference).
							Return(nil)
						return coursesMock
					}(),
				},
				Body:         request,
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, configGinCourseRouter)
	})
}

func TestUploadVOD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/uploadVOD", func(t *testing.T) {
		baseUrl := fmt.Sprintf("/api/course/%d/uploadVOD", testutils.CourseFPV.ID)
		url := fmt.Sprintf("%s?start=2022-07-04T10:00:00.000Z&title=VOD1", baseUrl)

		ctrl := gomock.NewController(t)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            baseUrl,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": {
				Method:         http.MethodPost,
				Url:            baseUrl,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid query": {
				Method:         http.MethodPost,
				Url:            baseUrl,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"can not create stream": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(ctrl)
						streamsMock.
							EXPECT().
							CreateStream(gomock.Any()).
							Return(errors.New(""))
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"can note create upload key": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(ctrl)
						streamsMock.
							EXPECT().
							CreateStream(gomock.Any()).
							Return(nil)
						return streamsMock
					}(),
					UploadKeyDao: func() dao.UploadKeyDao {
						streamsMock := mock_dao.NewMockUploadKeyDao(ctrl)
						streamsMock.
							EXPECT().
							CreateUploadKey(gomock.Any(), gomock.Any()).
							Return(errors.New(""))
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"no workers available": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao:   testutils.GetCoursesMock(t),
					StreamsDao:   testutils.GetStreamMock(t),
					UploadKeyDao: testutils.GetUploadKeyMock(t),
					WorkerDao: func() dao.WorkerDao {
						streamsMock := mock_dao.NewMockWorkerDao(ctrl)
						streamsMock.
							EXPECT().
							GetAliveWorkers().
							Return([]model.Worker{})
						return streamsMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			/*
				TODO: Prevent p.ServeHTTP
				"success": {
					Method:         http.MethodPost,
					Url:            url,
					TumLiveContext: &testutils.TUMLiveContextAdmin,
					DaoWrapper: dao.DaoWrapper{
						CoursesDao:   testutils.GetCoursesMock(t),
						StreamsDao:   testutils.GetStreamMock(t),
						UploadKeyDao: testutils.GetUploadKeyMock(t),
						WorkerDao: func() dao.WorkerDao {
							streamsMock := mock_dao.NewMockWorkerDao(ctrl)
							streamsMock.
								EXPECT().
								GetAliveWorkers().
								Return([]model.Worker{testutils.Worker1})
							return streamsMock
						}(),
					},
					ExpectedCode: http.StatusOK,
				},
			*/
		}
		testCases.Run(t, configGinCourseRouter)
	})

}
