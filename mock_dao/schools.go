// Code generated by MockGen. DO NOT EDIT.
// Source: schools.go

// Package mock_dao is a generated GoMock package.
package mock_dao

import (
	context "context"
	reflect "reflect"

	model "github.com/TUM-Dev/gocast/model"
	gomock "github.com/golang/mock/gomock"
)

// MockSchoolsDao is a mock of SchoolsDao interface.
type MockSchoolsDao struct {
	ctrl     *gomock.Controller
	recorder *MockSchoolsDaoMockRecorder
}

// MockSchoolsDaoMockRecorder is the mock recorder for MockSchoolsDao.
type MockSchoolsDaoMockRecorder struct {
	mock *MockSchoolsDao
}

// NewMockSchoolsDao creates a new mock instance.
func NewMockSchoolsDao(ctrl *gomock.Controller) *MockSchoolsDao {
	mock := &MockSchoolsDao{ctrl: ctrl}
	mock.recorder = &MockSchoolsDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSchoolsDao) EXPECT() *MockSchoolsDaoMockRecorder {
	return m.recorder
}

// AddAdmin mocks base method.
func (m *MockSchoolsDao) AddAdmin(arg0 context.Context, arg1 *model.School, arg2 *model.User) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddAdmin", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddAdmin indicates an expected call of AddAdmin.
func (mr *MockSchoolsDaoMockRecorder) AddAdmin(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddAdmin", reflect.TypeOf((*MockSchoolsDao)(nil).AddAdmin), arg0, arg1, arg2)
}

// Create mocks base method.
func (m *MockSchoolsDao) Create(arg0 context.Context, arg1 *model.School) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockSchoolsDaoMockRecorder) Create(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockSchoolsDao)(nil).Create), arg0, arg1)
}

// Delete mocks base method.
func (m *MockSchoolsDao) Delete(arg0 context.Context, arg1 uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockSchoolsDaoMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockSchoolsDao)(nil).Delete), arg0, arg1)
}

// Get mocks base method.
func (m *MockSchoolsDao) Get(arg0 context.Context, arg1 uint) (model.School, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(model.School)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockSchoolsDaoMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockSchoolsDao)(nil).Get), arg0, arg1)
}

// GetAdminCount mocks base method.
func (m *MockSchoolsDao) GetAdminCount(arg0 context.Context, arg1 uint) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAdminCount", arg0, arg1)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAdminCount indicates an expected call of GetAdminCount.
func (mr *MockSchoolsDaoMockRecorder) GetAdminCount(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAdminCount", reflect.TypeOf((*MockSchoolsDao)(nil).GetAdminCount), arg0, arg1)
}

// GetAdministeredSchoolsByUser mocks base method.
func (m *MockSchoolsDao) GetAdministeredSchoolsByUser(arg0 context.Context, arg1 *model.User) ([]model.School, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAdministeredSchoolsByUser", arg0, arg1)
	ret0, _ := ret[0].([]model.School)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAdministeredSchoolsByUser indicates an expected call of GetAdministeredSchoolsByUser.
func (mr *MockSchoolsDaoMockRecorder) GetAdministeredSchoolsByUser(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAdministeredSchoolsByUser", reflect.TypeOf((*MockSchoolsDao)(nil).GetAdministeredSchoolsByUser), arg0, arg1)
}

// GetAdmins mocks base method.
func (m *MockSchoolsDao) GetAdmins(arg0 context.Context, arg1 uint) ([]model.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAdmins", arg0, arg1)
	ret0, _ := ret[0].([]model.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAdmins indicates an expected call of GetAdmins.
func (mr *MockSchoolsDaoMockRecorder) GetAdmins(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAdmins", reflect.TypeOf((*MockSchoolsDao)(nil).GetAdmins), arg0, arg1)
}

// GetAdminsBySchoolAndUniversity mocks base method.
func (m *MockSchoolsDao) GetAdminsBySchoolAndUniversity(arg0 context.Context, arg1, arg2 string) ([]model.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAdminsBySchoolAndUniversity", arg0, arg1, arg2)
	ret0, _ := ret[0].([]model.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAdminsBySchoolAndUniversity indicates an expected call of GetAdminsBySchoolAndUniversity.
func (mr *MockSchoolsDaoMockRecorder) GetAdminsBySchoolAndUniversity(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAdminsBySchoolAndUniversity", reflect.TypeOf((*MockSchoolsDao)(nil).GetAdminsBySchoolAndUniversity), arg0, arg1, arg2)
}

// GetAll mocks base method.
func (m *MockSchoolsDao) GetAll() []model.School {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAll")
	ret0, _ := ret[0].([]model.School)
	return ret0
}

// GetAll indicates an expected call of GetAll.
func (mr *MockSchoolsDaoMockRecorder) GetAll() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAll", reflect.TypeOf((*MockSchoolsDao)(nil).GetAll))
}

// GetByNameAndUniversity mocks base method.
func (m *MockSchoolsDao) GetByNameAndUniversity(arg0 context.Context, arg1, arg2 string) (model.School, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByNameAndUniversity", arg0, arg1, arg2)
	ret0, _ := ret[0].(model.School)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByNameAndUniversity indicates an expected call of GetByNameAndUniversity.
func (mr *MockSchoolsDaoMockRecorder) GetByNameAndUniversity(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByNameAndUniversity", reflect.TypeOf((*MockSchoolsDao)(nil).GetByNameAndUniversity), arg0, arg1, arg2)
}

// Query mocks base method.
func (m *MockSchoolsDao) Query(arg0 context.Context, arg1 string) ([]model.School, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Query", arg0, arg1)
	ret0, _ := ret[0].([]model.School)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Query indicates an expected call of Query.
func (mr *MockSchoolsDaoMockRecorder) Query(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockSchoolsDao)(nil).Query), arg0, arg1)
}

// QueryAdministerdSchools mocks base method.
func (m *MockSchoolsDao) QueryAdministerdSchools(arg0 context.Context, arg1 *model.User, arg2 string) ([]model.School, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryAdministerdSchools", arg0, arg1, arg2)
	ret0, _ := ret[0].([]model.School)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryAdministerdSchools indicates an expected call of QueryAdministerdSchools.
func (mr *MockSchoolsDaoMockRecorder) QueryAdministerdSchools(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryAdministerdSchools", reflect.TypeOf((*MockSchoolsDao)(nil).QueryAdministerdSchools), arg0, arg1, arg2)
}

// RemoveAdmin mocks base method.
func (m *MockSchoolsDao) RemoveAdmin(arg0 context.Context, arg1, arg2 uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveAdmin", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveAdmin indicates an expected call of RemoveAdmin.
func (mr *MockSchoolsDaoMockRecorder) RemoveAdmin(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveAdmin", reflect.TypeOf((*MockSchoolsDao)(nil).RemoveAdmin), arg0, arg1, arg2)
}

// Update mocks base method.
func (m *MockSchoolsDao) Update(arg0 context.Context, arg1 *model.School) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockSchoolsDaoMockRecorder) Update(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockSchoolsDao)(nil).Update), arg0, arg1)
}