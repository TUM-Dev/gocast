// Code generated by MockGen. DO NOT EDIT.
// Source: email.go

// Package mock_dao is a generated GoMock package.
package mock_dao

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	model "github.com/TUM-Dev/gocast/model"
)

// MockEmailDao is a mock of EmailDao interface.
type MockEmailDao struct {
	ctrl     *gomock.Controller
	recorder *MockEmailDaoMockRecorder
}

// MockEmailDaoMockRecorder is the mock recorder for MockEmailDao.
type MockEmailDaoMockRecorder struct {
	mock *MockEmailDao
}

// NewMockEmailDao creates a new mock instance.
func NewMockEmailDao(ctrl *gomock.Controller) *MockEmailDao {
	mock := &MockEmailDao{ctrl: ctrl}
	mock.recorder = &MockEmailDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEmailDao) EXPECT() *MockEmailDaoMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockEmailDao) Create(arg0 context.Context, arg1 *model.Email) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockEmailDaoMockRecorder) Create(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockEmailDao)(nil).Create), arg0, arg1)
}

// Delete mocks base method.
func (m *MockEmailDao) Delete(arg0 context.Context, arg1 uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockEmailDaoMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockEmailDao)(nil).Delete), arg0, arg1)
}

// Get mocks base method.
func (m *MockEmailDao) Get(arg0 context.Context, arg1 uint) (model.Email, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(model.Email)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockEmailDaoMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockEmailDao)(nil).Get), arg0, arg1)
}

// GetDue mocks base method.
func (m *MockEmailDao) GetDue(arg0 context.Context, arg1 int) ([]model.Email, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDue", arg0, arg1)
	ret0, _ := ret[0].([]model.Email)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDue indicates an expected call of GetDue.
func (mr *MockEmailDaoMockRecorder) GetDue(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDue", reflect.TypeOf((*MockEmailDao)(nil).GetDue), arg0, arg1)
}

// GetFailed mocks base method.
func (m *MockEmailDao) GetFailed(arg0 context.Context) ([]model.Email, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetFailed", arg0)
	ret0, _ := ret[0].([]model.Email)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetFailed indicates an expected call of GetFailed.
func (mr *MockEmailDaoMockRecorder) GetFailed(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetFailed", reflect.TypeOf((*MockEmailDao)(nil).GetFailed), arg0)
}

// Save mocks base method.
func (m *MockEmailDao) Save(arg0 context.Context, arg1 *model.Email) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Save", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Save indicates an expected call of Save.
func (mr *MockEmailDaoMockRecorder) Save(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Save", reflect.TypeOf((*MockEmailDao)(nil).Save), arg0, arg1)
}
