// Code generated by MockGen. DO NOT EDIT.
// Source: lecture_halls.go

// Package mock_dao is a generated GoMock package.
package mock_dao

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	dao "github.com/joschahenningsen/TUM-Live/dao"
	model "github.com/joschahenningsen/TUM-Live/model"
)

// MockLectureHallsDao is a mock of LectureHallsDao interface.
type MockLectureHallsDao struct {
	ctrl     *gomock.Controller
	recorder *MockLectureHallsDaoMockRecorder
}

// MockLectureHallsDaoMockRecorder is the mock recorder for MockLectureHallsDao.
type MockLectureHallsDaoMockRecorder struct {
	mock *MockLectureHallsDao
}

// NewMockLectureHallsDao creates a new mock instance.
func NewMockLectureHallsDao(ctrl *gomock.Controller) *MockLectureHallsDao {
	mock := &MockLectureHallsDao{ctrl: ctrl}
	mock.recorder = &MockLectureHallsDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLectureHallsDao) EXPECT() *MockLectureHallsDaoMockRecorder {
	return m.recorder
}

// CreateLectureHall mocks base method.
func (m *MockLectureHallsDao) CreateLectureHall(lectureHall model.LectureHall) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CreateLectureHall", lectureHall)
}

// CreateLectureHall indicates an expected call of CreateLectureHall.
func (mr *MockLectureHallsDaoMockRecorder) CreateLectureHall(lectureHall interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateLectureHall", reflect.TypeOf((*MockLectureHallsDao)(nil).CreateLectureHall), lectureHall)
}

// DeleteLectureHall mocks base method.
func (m *MockLectureHallsDao) DeleteLectureHall(id uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteLectureHall", id)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteLectureHall indicates an expected call of DeleteLectureHall.
func (mr *MockLectureHallsDaoMockRecorder) DeleteLectureHall(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteLectureHall", reflect.TypeOf((*MockLectureHallsDao)(nil).DeleteLectureHall), id)
}

// FindPreset mocks base method.
func (m *MockLectureHallsDao) FindPreset(lectureHallID, presetID string) (model.CameraPreset, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindPreset", lectureHallID, presetID)
	ret0, _ := ret[0].(model.CameraPreset)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindPreset indicates an expected call of FindPreset.
func (mr *MockLectureHallsDaoMockRecorder) FindPreset(lectureHallID, presetID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindPreset", reflect.TypeOf((*MockLectureHallsDao)(nil).FindPreset), lectureHallID, presetID)
}

// GetAllLectureHalls mocks base method.
func (m *MockLectureHallsDao) GetAllLectureHalls() []model.LectureHall {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllLectureHalls")
	ret0, _ := ret[0].([]model.LectureHall)
	return ret0
}

// GetAllLectureHalls indicates an expected call of GetAllLectureHalls.
func (mr *MockLectureHallsDaoMockRecorder) GetAllLectureHalls() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllLectureHalls", reflect.TypeOf((*MockLectureHallsDao)(nil).GetAllLectureHalls))
}

// GetLectureHallByID mocks base method.
func (m *MockLectureHallsDao) GetLectureHallByID(id uint) (model.LectureHall, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLectureHallByID", id)
	ret0, _ := ret[0].(model.LectureHall)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLectureHallByID indicates an expected call of GetLectureHallByID.
func (mr *MockLectureHallsDaoMockRecorder) GetLectureHallByID(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLectureHallByID", reflect.TypeOf((*MockLectureHallsDao)(nil).GetLectureHallByID), id)
}

// GetLectureHallByPartialName mocks base method.
func (m *MockLectureHallsDao) GetLectureHallByPartialName(name string) (model.LectureHall, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLectureHallByPartialName", name)
	ret0, _ := ret[0].(model.LectureHall)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLectureHallByPartialName indicates an expected call of GetLectureHallByPartialName.
func (mr *MockLectureHallsDaoMockRecorder) GetLectureHallByPartialName(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLectureHallByPartialName", reflect.TypeOf((*MockLectureHallsDao)(nil).GetLectureHallByPartialName), name)
}

// GetStreamsForLectureHallIcal mocks base method.
func (m *MockLectureHallsDao) GetStreamsForLectureHallIcal(userId uint, lectureHalls []uint, all bool) ([]dao.CalendarResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStreamsForLectureHallIcal", userId, lectureHalls, all)
	ret0, _ := ret[0].([]dao.CalendarResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetStreamsForLectureHallIcal indicates an expected call of GetStreamsForLectureHallIcal.
func (mr *MockLectureHallsDaoMockRecorder) GetStreamsForLectureHallIcal(userId, lectureHalls, all interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStreamsForLectureHallIcal", reflect.TypeOf((*MockLectureHallsDao)(nil).GetStreamsForLectureHallIcal), userId, lectureHalls, all)
}

// SaveLectureHall mocks base method.
func (m *MockLectureHallsDao) SaveLectureHall(lectureHall model.LectureHall) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveLectureHall", lectureHall)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveLectureHall indicates an expected call of SaveLectureHall.
func (mr *MockLectureHallsDaoMockRecorder) SaveLectureHall(lectureHall interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveLectureHall", reflect.TypeOf((*MockLectureHallsDao)(nil).SaveLectureHall), lectureHall)
}

// SaveLectureHallFullAssoc mocks base method.
func (m *MockLectureHallsDao) SaveLectureHallFullAssoc(lectureHall model.LectureHall) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SaveLectureHallFullAssoc", lectureHall)
}

// SaveLectureHallFullAssoc indicates an expected call of SaveLectureHallFullAssoc.
func (mr *MockLectureHallsDaoMockRecorder) SaveLectureHallFullAssoc(lectureHall interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveLectureHallFullAssoc", reflect.TypeOf((*MockLectureHallsDao)(nil).SaveLectureHallFullAssoc), lectureHall)
}

// SavePreset mocks base method.
func (m *MockLectureHallsDao) SavePreset(preset model.CameraPreset) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SavePreset", preset)
	ret0, _ := ret[0].(error)
	return ret0
}

// SavePreset indicates an expected call of SavePreset.
func (mr *MockLectureHallsDaoMockRecorder) SavePreset(preset interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SavePreset", reflect.TypeOf((*MockLectureHallsDao)(nil).SavePreset), preset)
}

// UnsetDefaults mocks base method.
func (m *MockLectureHallsDao) UnsetDefaults(lectureHallID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnsetDefaults", lectureHallID)
	ret0, _ := ret[0].(error)
	return ret0
}

// UnsetDefaults indicates an expected call of UnsetDefaults.
func (mr *MockLectureHallsDaoMockRecorder) UnsetDefaults(lectureHallID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetDefaults", reflect.TypeOf((*MockLectureHallsDao)(nil).UnsetDefaults), lectureHallID)
}
