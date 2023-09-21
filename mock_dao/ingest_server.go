// Code generated by MockGen. DO NOT EDIT.
// Source: ingest_server.go

// Package mock_dao is a generated GoMock package.
package mock_dao

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	model "github.com/TUM-Dev/gocast/model"
)

// MockIngestServerDao is a mock of IngestServerDao interface.
type MockIngestServerDao struct {
	ctrl     *gomock.Controller
	recorder *MockIngestServerDaoMockRecorder
}

// MockIngestServerDaoMockRecorder is the mock recorder for MockIngestServerDao.
type MockIngestServerDaoMockRecorder struct {
	mock *MockIngestServerDao
}

// NewMockIngestServerDao creates a new mock instance.
func NewMockIngestServerDao(ctrl *gomock.Controller) *MockIngestServerDao {
	mock := &MockIngestServerDao{ctrl: ctrl}
	mock.recorder = &MockIngestServerDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIngestServerDao) EXPECT() *MockIngestServerDaoMockRecorder {
	return m.recorder
}

// GetBestIngestServer mocks base method.
func (m *MockIngestServerDao) GetBestIngestServer() (model.IngestServer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBestIngestServer")
	ret0, _ := ret[0].(model.IngestServer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBestIngestServer indicates an expected call of GetBestIngestServer.
func (mr *MockIngestServerDaoMockRecorder) GetBestIngestServer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBestIngestServer", reflect.TypeOf((*MockIngestServerDao)(nil).GetBestIngestServer))
}

// GetStreamSlot mocks base method.
func (m *MockIngestServerDao) GetStreamSlot(ingestServerID uint) (model.StreamName, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStreamSlot", ingestServerID)
	ret0, _ := ret[0].(model.StreamName)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetStreamSlot indicates an expected call of GetStreamSlot.
func (mr *MockIngestServerDaoMockRecorder) GetStreamSlot(ingestServerID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStreamSlot", reflect.TypeOf((*MockIngestServerDao)(nil).GetStreamSlot), ingestServerID)
}

// GetTranscodedStreamSlot mocks base method.
func (m *MockIngestServerDao) GetTranscodedStreamSlot(ingestServerID uint) (model.StreamName, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTranscodedStreamSlot", ingestServerID)
	ret0, _ := ret[0].(model.StreamName)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTranscodedStreamSlot indicates an expected call of GetTranscodedStreamSlot.
func (mr *MockIngestServerDaoMockRecorder) GetTranscodedStreamSlot(ingestServerID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTranscodedStreamSlot", reflect.TypeOf((*MockIngestServerDao)(nil).GetTranscodedStreamSlot), ingestServerID)
}

// RemoveStreamFromSlot mocks base method.
func (m *MockIngestServerDao) RemoveStreamFromSlot(streamID uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveStreamFromSlot", streamID)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveStreamFromSlot indicates an expected call of RemoveStreamFromSlot.
func (mr *MockIngestServerDaoMockRecorder) RemoveStreamFromSlot(streamID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveStreamFromSlot", reflect.TypeOf((*MockIngestServerDao)(nil).RemoveStreamFromSlot), streamID)
}

// SaveIngestServer mocks base method.
func (m *MockIngestServerDao) SaveIngestServer(server model.IngestServer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SaveIngestServer", server)
}

// SaveIngestServer indicates an expected call of SaveIngestServer.
func (mr *MockIngestServerDaoMockRecorder) SaveIngestServer(server interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveIngestServer", reflect.TypeOf((*MockIngestServerDao)(nil).SaveIngestServer), server)
}

// SaveSlot mocks base method.
func (m *MockIngestServerDao) SaveSlot(slot model.StreamName) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SaveSlot", slot)
}

// SaveSlot indicates an expected call of SaveSlot.
func (mr *MockIngestServerDaoMockRecorder) SaveSlot(slot interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveSlot", reflect.TypeOf((*MockIngestServerDao)(nil).SaveSlot), slot)
}
