package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
	"time"
)

func TestCoursesCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

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
			/*"success": {
				Method: http.MethodDelete,
				Url:    url,
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
							DeleteCourse(gomock.Any())
						return coursesMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
			},*/
		}

		testCases.Run(t, configGinCourseRouter)
	})
}

func TestCoursesLectureActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

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
			"invalid body": {
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
						return coursesMock
					}(),
				},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"lectureHallId set on 'premiere'": {
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
						return coursesMock
					}(),
				},
				Body: bytes.NewBuffer(
					testutils.First(
						json.Marshal(
							createLectureRequest{
								LectureHallId: "1",
								Premiere:      true,
							},
						)).([]byte)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid lectureHallId": {
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
						return coursesMock
					}(),
				},
				Body: bytes.NewBuffer(
					testutils.First(
						json.Marshal(
							createLectureRequest{
								Title:         "Lecture 1",
								LectureHallId: "abc",
								Start:         time.Now(),
								Duration:      90,
								Premiere:      false,
								Vodup:         false,
								DateSeries:    []time.Time{},
							},
						)).([]byte)),
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
				},
				Body: bytes.NewBuffer(
					testutils.First(
						json.Marshal(
							createLectureRequest{
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
						)).([]byte)),
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
				},
				Body: bytes.NewBuffer(
					testutils.First(
						json.Marshal(
							createLectureRequest{
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
						)).([]byte)),
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
			"invalid body": {
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
						return coursesMock
					}(),
				},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid stream id in body": {
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
						return coursesMock
					}(),
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
				Body: bytes.NewBuffer(testutils.First(
					json.Marshal(deleteLecturesRequest{StreamIDs: []string{
						fmt.Sprintf("%d", testutils.StreamGBSLive.ID)},
					})).([]byte)),
				ExpectedCode: http.StatusForbidden,
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
						return coursesMock
					}(),
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
				},
				Body: bytes.NewBuffer(testutils.First(
					json.Marshal(deleteLecturesRequest{StreamIDs: []string{
						fmt.Sprintf("%d", testutils.StreamFPVLive.ID)},
					})).([]byte)),
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
			"invalid streamID": {
				Method:         http.MethodPost,
				Url:            fmt.Sprintf("/api/course/%d/renameLecture/abc", testutils.CourseFPV.ID),
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
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid body": {
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
						return coursesMock
					}(),
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
				Body: bytes.NewBuffer(
					testutils.First(json.Marshal(renameLectureRequest{
						Name: "Proofs #1",
					})).([]byte)),
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
				Body: bytes.NewBuffer(
					testutils.First(json.Marshal(renameLectureRequest{
						Name: "Proofs #1",
					})).([]byte)),
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
						coursesMock.EXPECT().
							CreateCourse(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
						return coursesMock
					}(),
				},
				Body: bytes.NewBuffer(
					testutils.First(json.Marshal(renameLectureRequest{
						Name: "Proofs #1",
					})).([]byte)),
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
			"stream not found": {
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
						return coursesMock
					}(),
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
			"stream not found": {
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
							GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return coursesMock
					}(),
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
}
