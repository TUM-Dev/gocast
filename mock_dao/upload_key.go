// Code generated by MockGen. DO NOT EDIT.
// Source: upload_key.go

// Package mock_dao is a generated GoMock package.
package mock_dao

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	model "github.com/joschahenningsen/TUM-Live/model"
)

// MockUploadKeyDao is a mock of UploadKeyDao interface.
type MockUploadKeyDao struct {
	ctrl     *gomock.Controller
	recorder *MockUploadKeyDaoMockRecorder
}

// MockUploadKeyDaoMockRecorder is the mock recorder for MockUploadKeyDao.
type MockUploadKeyDaoMockRecorder struct {
	mock *MockUploadKeyDao
}

// NewMockUploadKeyDao creates a new mock instance.
func NewMockUploadKeyDao(ctrl *gomock.Controller) *MockUploadKeyDao {
	mock := &MockUploadKeyDao{ctrl: ctrl}
	mock.recorder = &MockUploadKeyDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockUploadKeyDao) EXPECT() *MockUploadKeyDaoMockRecorder {
	return m.recorder
}

// CreateUploadKey mocks base method.
func (m *MockUploadKeyDao) CreateUploadKey(key string, stream uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateUploadKey", key, stream)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateUploadKey indicates an expected call of CreateUploadKey.
func (mr *MockUploadKeyDaoMockRecorder) CreateUploadKey(key, stream interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateUploadKey", reflect.TypeOf((*MockUploadKeyDao)(nil).CreateUploadKey), key, stream)
}

// DeleteUploadKey mocks base method.
func (m *MockUploadKeyDao) DeleteUploadKey(key model.UploadKey) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteUploadKey", key)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteUploadKey indicates an expected call of DeleteUploadKey.
func (mr *MockUploadKeyDaoMockRecorder) DeleteUploadKey(key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteUploadKey", reflect.TypeOf((*MockUploadKeyDao)(nil).DeleteUploadKey), key)
}

// GetUploadKey mocks base method.
func (m *MockUploadKeyDao) GetUploadKey(key string) (model.UploadKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUploadKey", key)
	ret0, _ := ret[0].(model.UploadKey)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUploadKey indicates an expected call of GetUploadKey.
func (mr *MockUploadKeyDaoMockRecorder) GetUploadKey(key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUploadKey", reflect.TypeOf((*MockUploadKeyDao)(nil).GetUploadKey), key)
}
