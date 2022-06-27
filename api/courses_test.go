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
			"UpdateCourse returns error": {
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
}
