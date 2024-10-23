// Code generated by MockGen. DO NOT EDIT.
// Source: courses.go

// Package mock_dao is a generated GoMock package.
package mock_dao

import (
	context "context"
	reflect "reflect"

	dao "github.com/TUM-Dev/gocast/dao"
	model "github.com/TUM-Dev/gocast/model"
	gomock "github.com/golang/mock/gomock"
)

// MockCoursesDao is a mock of CoursesDao interface.
type MockCoursesDao struct {
	ctrl     *gomock.Controller
	recorder *MockCoursesDaoMockRecorder
}

// MockCoursesDaoMockRecorder is the mock recorder for MockCoursesDao.
type MockCoursesDaoMockRecorder struct {
	mock *MockCoursesDao
}

// NewMockCoursesDao creates a new mock instance.
func NewMockCoursesDao(ctrl *gomock.Controller) *MockCoursesDao {
	mock := &MockCoursesDao{ctrl: ctrl}
	mock.recorder = &MockCoursesDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCoursesDao) EXPECT() *MockCoursesDaoMockRecorder {
	return m.recorder
}

// AddAdminToCourse mocks base method.
func (m *MockCoursesDao) AddAdminToCourse(userID, courseID uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddAdminToCourse", userID, courseID)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddAdminToCourse indicates an expected call of AddAdminToCourse.
func (mr *MockCoursesDaoMockRecorder) AddAdminToCourse(userID, courseID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddAdminToCourse", reflect.TypeOf((*MockCoursesDao)(nil).AddAdminToCourse), userID, courseID)
}

// CreateCourse mocks base method.
func (m *MockCoursesDao) CreateCourse(ctx context.Context, course *model.Course, keep bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateCourse", ctx, course, keep)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateCourse indicates an expected call of CreateCourse.
func (mr *MockCoursesDaoMockRecorder) CreateCourse(ctx, course, keep interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateCourse", reflect.TypeOf((*MockCoursesDao)(nil).CreateCourse), ctx, course, keep)
}

// DeleteCourse mocks base method.
func (m *MockCoursesDao) DeleteCourse(course model.Course) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeleteCourse", course)
}

// DeleteCourse indicates an expected call of DeleteCourse.
func (mr *MockCoursesDaoMockRecorder) DeleteCourse(course interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCourse", reflect.TypeOf((*MockCoursesDao)(nil).DeleteCourse), course)
}

// GetAdministeredCoursesByUserId mocks base method.
func (m *MockCoursesDao) GetAdministeredCoursesByUserId(ctx context.Context, userid uint, teachingTerm string, year int) ([]model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAdministeredCoursesByUserId", ctx, userid, teachingTerm, year)
	ret0, _ := ret[0].([]model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAdministeredCoursesByUserId indicates an expected call of GetAdministeredCoursesByUserId.
func (mr *MockCoursesDaoMockRecorder) GetAdministeredCoursesByUserId(ctx, userid, teachingTerm, year interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAdministeredCoursesByUserId", reflect.TypeOf((*MockCoursesDao)(nil).GetAdministeredCoursesByUserId), ctx, userid, teachingTerm, year)
}

// GetAllCourses mocks base method.
func (m *MockCoursesDao) GetAllCourses() ([]model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllCourses")
	ret0, _ := ret[0].([]model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllCourses indicates an expected call of GetAllCourses.
func (mr *MockCoursesDaoMockRecorder) GetAllCourses() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllCourses", reflect.TypeOf((*MockCoursesDao)(nil).GetAllCourses))
}

// GetAllCoursesForSemester mocks base method.
func (m *MockCoursesDao) GetAllCoursesForSemester(year int, term string, ctx context.Context) []model.Course {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllCoursesForSemester", year, term, ctx)
	ret0, _ := ret[0].([]model.Course)
	return ret0
}

// GetAllCoursesForSemester indicates an expected call of GetAllCoursesForSemester.
func (mr *MockCoursesDaoMockRecorder) GetAllCoursesForSemester(year, term, ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllCoursesForSemester", reflect.TypeOf((*MockCoursesDao)(nil).GetAllCoursesForSemester), year, term, ctx)
}

// GetAllCoursesWithTUMIDFromSemester mocks base method.
func (m *MockCoursesDao) GetAllCoursesWithTUMIDFromSemester(ctx context.Context, year int, term string) ([]model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllCoursesWithTUMIDFromSemester", ctx, year, term)
	ret0, _ := ret[0].([]model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllCoursesWithTUMIDFromSemester indicates an expected call of GetAllCoursesWithTUMIDFromSemester.
func (mr *MockCoursesDaoMockRecorder) GetAllCoursesWithTUMIDFromSemester(ctx, year, term interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllCoursesWithTUMIDFromSemester", reflect.TypeOf((*MockCoursesDao)(nil).GetAllCoursesWithTUMIDFromSemester), ctx, year, term)
}

// GetAvailableSemesters mocks base method.
func (m *MockCoursesDao) GetAvailableSemesters(c context.Context) []dao.Semester {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAvailableSemesters", c)
	ret0, _ := ret[0].([]dao.Semester)
	return ret0
}

// GetAvailableSemesters indicates an expected call of GetAvailableSemesters.
func (mr *MockCoursesDaoMockRecorder) GetAvailableSemesters(c interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAvailableSemesters", reflect.TypeOf((*MockCoursesDao)(nil).GetAvailableSemesters), c)
}

// GetCourseAdmins mocks base method.
func (m *MockCoursesDao) GetCourseAdmins(courseID uint) ([]model.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCourseAdmins", courseID)
	ret0, _ := ret[0].([]model.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCourseAdmins indicates an expected call of GetCourseAdmins.
func (mr *MockCoursesDaoMockRecorder) GetCourseAdmins(courseID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCourseAdmins", reflect.TypeOf((*MockCoursesDao)(nil).GetCourseAdmins), courseID)
}

// GetCourseById mocks base method.
func (m *MockCoursesDao) GetCourseById(ctx context.Context, id uint) (model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCourseById", ctx, id)
	ret0, _ := ret[0].(model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCourseById indicates an expected call of GetCourseById.
func (mr *MockCoursesDaoMockRecorder) GetCourseById(ctx, id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCourseById", reflect.TypeOf((*MockCoursesDao)(nil).GetCourseById), ctx, id)
}

// GetCourseByShortLink mocks base method.
func (m *MockCoursesDao) GetCourseByShortLink(link string) (model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCourseByShortLink", link)
	ret0, _ := ret[0].(model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCourseByShortLink indicates an expected call of GetCourseByShortLink.
func (mr *MockCoursesDaoMockRecorder) GetCourseByShortLink(link interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCourseByShortLink", reflect.TypeOf((*MockCoursesDao)(nil).GetCourseByShortLink), link)
}

// GetCourseBySlugYearAndTerm mocks base method.
func (m *MockCoursesDao) GetCourseBySlugYearAndTerm(ctx context.Context, slug, term string, year int) (model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCourseBySlugYearAndTerm", ctx, slug, term, year)
	ret0, _ := ret[0].(model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCourseBySlugYearAndTerm indicates an expected call of GetCourseBySlugYearAndTerm.
func (mr *MockCoursesDaoMockRecorder) GetCourseBySlugYearAndTerm(ctx, slug, term, year interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCourseBySlugYearAndTerm", reflect.TypeOf((*MockCoursesDao)(nil).GetCourseBySlugYearAndTerm), ctx, slug, term, year)
}

// GetCourseByToken mocks base method.
func (m *MockCoursesDao) GetCourseByToken(token string) (model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCourseByToken", token)
	ret0, _ := ret[0].(model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCourseByToken indicates an expected call of GetCourseByToken.
func (mr *MockCoursesDaoMockRecorder) GetCourseByToken(token interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCourseByToken", reflect.TypeOf((*MockCoursesDao)(nil).GetCourseByToken), token)
}

// GetCourseForLecturerIdByYearAndTerm mocks base method.
func (m *MockCoursesDao) GetCourseForLecturerIdByYearAndTerm(c context.Context, year int, term string, userId uint) ([]model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCourseForLecturerIdByYearAndTerm", c, year, term, userId)
	ret0, _ := ret[0].([]model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCourseForLecturerIdByYearAndTerm indicates an expected call of GetCourseForLecturerIdByYearAndTerm.
func (mr *MockCoursesDaoMockRecorder) GetCourseForLecturerIdByYearAndTerm(c, year, term, userId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCourseForLecturerIdByYearAndTerm", reflect.TypeOf((*MockCoursesDao)(nil).GetCourseForLecturerIdByYearAndTerm), c, year, term, userId)
}

// GetCurrentOrNextLectureForCourse mocks base method.
func (m *MockCoursesDao) GetCurrentOrNextLectureForCourse(ctx context.Context, courseID uint) (model.Stream, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCurrentOrNextLectureForCourse", ctx, courseID)
	ret0, _ := ret[0].(model.Stream)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCurrentOrNextLectureForCourse indicates an expected call of GetCurrentOrNextLectureForCourse.
func (mr *MockCoursesDaoMockRecorder) GetCurrentOrNextLectureForCourse(ctx, courseID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCurrentOrNextLectureForCourse", reflect.TypeOf((*MockCoursesDao)(nil).GetCurrentOrNextLectureForCourse), ctx, courseID)
}

// GetInvitedUsersForCourse mocks base method.
func (m *MockCoursesDao) GetInvitedUsersForCourse(course *model.Course) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInvitedUsersForCourse", course)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetInvitedUsersForCourse indicates an expected call of GetInvitedUsersForCourse.
func (mr *MockCoursesDaoMockRecorder) GetInvitedUsersForCourse(course interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInvitedUsersForCourse", reflect.TypeOf((*MockCoursesDao)(nil).GetInvitedUsersForCourse), course)
}

// GetPublicAndLoggedInCourses mocks base method.
func (m *MockCoursesDao) GetPublicAndLoggedInCourses(year int, term string) ([]model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPublicAndLoggedInCourses", year, term)
	ret0, _ := ret[0].([]model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPublicAndLoggedInCourses indicates an expected call of GetPublicAndLoggedInCourses.
func (mr *MockCoursesDaoMockRecorder) GetPublicAndLoggedInCourses(year, term interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPublicAndLoggedInCourses", reflect.TypeOf((*MockCoursesDao)(nil).GetPublicAndLoggedInCourses), year, term)
}

// GetPublicCourses mocks base method.
func (m *MockCoursesDao) GetPublicCourses(year int, term string) ([]model.Course, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPublicCourses", year, term)
	ret0, _ := ret[0].([]model.Course)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPublicCourses indicates an expected call of GetPublicCourses.
func (mr *MockCoursesDaoMockRecorder) GetPublicCourses(year, term interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPublicCourses", reflect.TypeOf((*MockCoursesDao)(nil).GetPublicCourses), year, term)
}

// RemoveAdminFromCourse mocks base method.
func (m *MockCoursesDao) RemoveAdminFromCourse(userID, courseID uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveAdminFromCourse", userID, courseID)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveAdminFromCourse indicates an expected call of RemoveAdminFromCourse.
func (mr *MockCoursesDaoMockRecorder) RemoveAdminFromCourse(userID, courseID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveAdminFromCourse", reflect.TypeOf((*MockCoursesDao)(nil).RemoveAdminFromCourse), userID, courseID)
}

// UnDeleteCourse mocks base method.
func (m *MockCoursesDao) UnDeleteCourse(ctx context.Context, course model.Course) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnDeleteCourse", ctx, course)
	ret0, _ := ret[0].(error)
	return ret0
}

// UnDeleteCourse indicates an expected call of UnDeleteCourse.
func (mr *MockCoursesDaoMockRecorder) UnDeleteCourse(ctx, course interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnDeleteCourse", reflect.TypeOf((*MockCoursesDao)(nil).UnDeleteCourse), ctx, course)
}

// UpdateCourse mocks base method.
func (m *MockCoursesDao) UpdateCourse(ctx context.Context, course model.Course) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateCourse", ctx, course)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateCourse indicates an expected call of UpdateCourse.
func (mr *MockCoursesDaoMockRecorder) UpdateCourse(ctx, course interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateCourse", reflect.TypeOf((*MockCoursesDao)(nil).UpdateCourse), ctx, course)
}

// UpdateCourseMetadata mocks base method.
func (m *MockCoursesDao) UpdateCourseMetadata(ctx context.Context, course model.Course) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateCourseMetadata", ctx, course)
}

// UpdateCourseMetadata indicates an expected call of UpdateCourseMetadata.
func (mr *MockCoursesDaoMockRecorder) UpdateCourseMetadata(ctx, course interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateCourseMetadata", reflect.TypeOf((*MockCoursesDao)(nil).UpdateCourseMetadata), ctx, course)
}
