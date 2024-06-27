// Code generated by MockGen. DO NOT EDIT.
// Source: worker.go

// Package mock_dao is a generated GoMock package.
package mock_dao

import (
	context "context"
	reflect "reflect"

	model "github.com/TUM-Dev/gocast/model"
	gomock "github.com/golang/mock/gomock"
)

// MockWorkerDao is a mock of WorkerDao interface.
type MockWorkerDao struct {
	ctrl     *gomock.Controller
	recorder *MockWorkerDaoMockRecorder
}

// MockWorkerDaoMockRecorder is the mock recorder for MockWorkerDao.
type MockWorkerDaoMockRecorder struct {
	mock *MockWorkerDao
}

// NewMockWorkerDao creates a new mock instance.
func NewMockWorkerDao(ctrl *gomock.Controller) *MockWorkerDao {
	mock := &MockWorkerDao{ctrl: ctrl}
	mock.recorder = &MockWorkerDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockWorkerDao) EXPECT() *MockWorkerDaoMockRecorder {
	return m.recorder
}

// CreateWorker mocks base method.
func (m *MockWorkerDao) CreateWorker(worker *model.Worker) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateWorker", worker)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateWorker indicates an expected call of CreateWorker.
func (mr *MockWorkerDaoMockRecorder) CreateWorker(worker interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateWorker", reflect.TypeOf((*MockWorkerDao)(nil).CreateWorker), worker)
}

// DeleteWorker mocks base method.
func (m *MockWorkerDao) DeleteWorker(workerID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteWorker", workerID)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteWorker indicates an expected call of DeleteWorker.
func (mr *MockWorkerDaoMockRecorder) DeleteWorker(workerID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteWorker", reflect.TypeOf((*MockWorkerDao)(nil).DeleteWorker), workerID)
}

// GetAliveWorkers mocks base method.
func (m *MockWorkerDao) GetAliveWorkers(arg0 uint) []model.Worker {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAliveWorkers", arg0)
	ret0, _ := ret[0].([]model.Worker)
	return ret0
}

// GetAliveWorkers indicates an expected call of GetAliveWorkers.
func (mr *MockWorkerDaoMockRecorder) GetAliveWorkers(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAliveWorkers", reflect.TypeOf((*MockWorkerDao)(nil).GetAliveWorkers), arg0)
}

// GetAllWorkers mocks base method.
func (m *MockWorkerDao) GetAllWorkers(arg0 []model.School) ([]model.Worker, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllWorkers", arg0)
	ret0, _ := ret[0].([]model.Worker)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllWorkers indicates an expected call of GetAllWorkers.
func (mr *MockWorkerDaoMockRecorder) GetAllWorkers(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllWorkers", reflect.TypeOf((*MockWorkerDao)(nil).GetAllWorkers), arg0)
}

// GetWorkerByHostname mocks base method.
func (m *MockWorkerDao) GetWorkerByHostname(ctx context.Context, address, hostname string) (model.Worker, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetWorkerByHostname", ctx, address, hostname)
	ret0, _ := ret[0].(model.Worker)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkerByHostname indicates an expected call of GetWorkerByHostname.
func (mr *MockWorkerDaoMockRecorder) GetWorkerByHostname(ctx, address, hostname interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetWorkerByHostname", reflect.TypeOf((*MockWorkerDao)(nil).GetWorkerByHostname), ctx, address, hostname)
}

// GetWorkerByID mocks base method.
func (m *MockWorkerDao) GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetWorkerByID", ctx, workerID)
	ret0, _ := ret[0].(model.Worker)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkerByID indicates an expected call of GetWorkerByID.
func (mr *MockWorkerDaoMockRecorder) GetWorkerByID(ctx, workerID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetWorkerByID", reflect.TypeOf((*MockWorkerDao)(nil).GetWorkerByID), ctx, workerID)
}

// SaveWorker mocks base method.
func (m *MockWorkerDao) SaveWorker(worker model.Worker) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveWorker", worker)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveWorker indicates an expected call of SaveWorker.
func (mr *MockWorkerDaoMockRecorder) SaveWorker(worker interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveWorker", reflect.TypeOf((*MockWorkerDao)(nil).SaveWorker), worker)
}
