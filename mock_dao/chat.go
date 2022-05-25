// Code generated by MockGen. DO NOT EDIT.
// Source: dao/chat.go

// Package mock_dao is a generated GoMock package.
package mock_dao

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	model "github.com/joschahenningsen/TUM-Live/model"
)

// MockChatDao is a mock of ChatDao interface.
type MockChatDao struct {
	ctrl     *gomock.Controller
	recorder *MockChatDaoMockRecorder
}

// MockChatDaoMockRecorder is the mock recorder for MockChatDao.
type MockChatDaoMockRecorder struct {
	mock *MockChatDao
}

// NewMockChatDao creates a new mock instance.
func NewMockChatDao(ctrl *gomock.Controller) *MockChatDao {
	mock := &MockChatDao{ctrl: ctrl}
	mock.recorder = &MockChatDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockChatDao) EXPECT() *MockChatDaoMockRecorder {
	return m.recorder
}

// AddChatPoll mocks base method.
func (m *MockChatDao) AddChatPoll(poll *model.Poll) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddChatPoll", poll)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddChatPoll indicates an expected call of AddChatPoll.
func (mr *MockChatDaoMockRecorder) AddChatPoll(poll interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddChatPoll", reflect.TypeOf((*MockChatDao)(nil).AddChatPoll), poll)
}

// AddChatPollOptionVote mocks base method.
func (m *MockChatDao) AddChatPollOptionVote(pollOptionId, userId uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddChatPollOptionVote", pollOptionId, userId)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddChatPollOptionVote indicates an expected call of AddChatPollOptionVote.
func (mr *MockChatDaoMockRecorder) AddChatPollOptionVote(pollOptionId, userId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddChatPollOptionVote", reflect.TypeOf((*MockChatDao)(nil).AddChatPollOptionVote), pollOptionId, userId)
}

// AddMessage mocks base method.
func (m *MockChatDao) AddMessage(chat *model.Chat) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddMessage", chat)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddMessage indicates an expected call of AddMessage.
func (mr *MockChatDaoMockRecorder) AddMessage(chat interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddMessage", reflect.TypeOf((*MockChatDao)(nil).AddMessage), chat)
}

// ApproveChat mocks base method.
func (m *MockChatDao) ApproveChat(id uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ApproveChat", id)
	ret0, _ := ret[0].(error)
	return ret0
}

// ApproveChat indicates an expected call of ApproveChat.
func (mr *MockChatDaoMockRecorder) ApproveChat(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ApproveChat", reflect.TypeOf((*MockChatDao)(nil).ApproveChat), id)
}

// CloseActivePoll mocks base method.
func (m *MockChatDao) CloseActivePoll(streamID uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseActivePoll", streamID)
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseActivePoll indicates an expected call of CloseActivePoll.
func (mr *MockChatDaoMockRecorder) CloseActivePoll(streamID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseActivePoll", reflect.TypeOf((*MockChatDao)(nil).CloseActivePoll), streamID)
}

// DeleteChat mocks base method.
func (m *MockChatDao) DeleteChat(id uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteChat", id)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteChat indicates an expected call of DeleteChat.
func (mr *MockChatDaoMockRecorder) DeleteChat(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteChat", reflect.TypeOf((*MockChatDao)(nil).DeleteChat), id)
}

// GetActivePoll mocks base method.
func (m *MockChatDao) GetActivePoll(streamID uint) (model.Poll, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActivePoll", streamID)
	ret0, _ := ret[0].(model.Poll)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetActivePoll indicates an expected call of GetActivePoll.
func (mr *MockChatDaoMockRecorder) GetActivePoll(streamID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActivePoll", reflect.TypeOf((*MockChatDao)(nil).GetActivePoll), streamID)
}

// GetAllChats mocks base method.
func (m *MockChatDao) GetAllChats(userID, streamID uint) ([]model.Chat, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllChats", userID, streamID)
	ret0, _ := ret[0].([]model.Chat)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllChats indicates an expected call of GetAllChats.
func (mr *MockChatDaoMockRecorder) GetAllChats(userID, streamID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllChats", reflect.TypeOf((*MockChatDao)(nil).GetAllChats), userID, streamID)
}

// GetChatUsers mocks base method.
func (m *MockChatDao) GetChatUsers(streamid uint) ([]model.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChatUsers", streamid)
	ret0, _ := ret[0].([]model.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetChatUsers indicates an expected call of GetChatUsers.
func (mr *MockChatDaoMockRecorder) GetChatUsers(streamid interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChatUsers", reflect.TypeOf((*MockChatDao)(nil).GetChatUsers), streamid)
}

// GetChatsByUser mocks base method.
func (m *MockChatDao) GetChatsByUser(userID uint) ([]model.Chat, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChatsByUser", userID)
	ret0, _ := ret[0].([]model.Chat)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetChatsByUser indicates an expected call of GetChatsByUser.
func (mr *MockChatDaoMockRecorder) GetChatsByUser(userID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChatsByUser", reflect.TypeOf((*MockChatDao)(nil).GetChatsByUser), userID)
}

// GetNumLikes mocks base method.
func (m *MockChatDao) GetNumLikes(chatID uint) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNumLikes", chatID)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNumLikes indicates an expected call of GetNumLikes.
func (mr *MockChatDaoMockRecorder) GetNumLikes(chatID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNumLikes", reflect.TypeOf((*MockChatDao)(nil).GetNumLikes), chatID)
}

// GetPollOptionVoteCount mocks base method.
func (m *MockChatDao) GetPollOptionVoteCount(pollOptionId uint) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPollOptionVoteCount", pollOptionId)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPollOptionVoteCount indicates an expected call of GetPollOptionVoteCount.
func (mr *MockChatDaoMockRecorder) GetPollOptionVoteCount(pollOptionId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPollOptionVoteCount", reflect.TypeOf((*MockChatDao)(nil).GetPollOptionVoteCount), pollOptionId)
}

// GetPollUserVote mocks base method.
func (m *MockChatDao) GetPollUserVote(pollId, userId uint) (uint, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPollUserVote", pollId, userId)
	ret0, _ := ret[0].(uint)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPollUserVote indicates an expected call of GetPollUserVote.
func (mr *MockChatDaoMockRecorder) GetPollUserVote(pollId, userId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPollUserVote", reflect.TypeOf((*MockChatDao)(nil).GetPollUserVote), pollId, userId)
}

// GetVisibleChats mocks base method.
func (m *MockChatDao) GetVisibleChats(userID, streamID uint) ([]model.Chat, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVisibleChats", userID, streamID)
	ret0, _ := ret[0].([]model.Chat)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVisibleChats indicates an expected call of GetVisibleChats.
func (mr *MockChatDaoMockRecorder) GetVisibleChats(userID, streamID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVisibleChats", reflect.TypeOf((*MockChatDao)(nil).GetVisibleChats), userID, streamID)
}

// ResolveChat mocks base method.
func (m *MockChatDao) ResolveChat(id uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResolveChat", id)
	ret0, _ := ret[0].(error)
	return ret0
}

// ResolveChat indicates an expected call of ResolveChat.
func (mr *MockChatDaoMockRecorder) ResolveChat(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResolveChat", reflect.TypeOf((*MockChatDao)(nil).ResolveChat), id)
}

// ToggleLike mocks base method.
func (m *MockChatDao) ToggleLike(userID, chatID uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ToggleLike", userID, chatID)
	ret0, _ := ret[0].(error)
	return ret0
}

// ToggleLike indicates an expected call of ToggleLike.
func (mr *MockChatDaoMockRecorder) ToggleLike(userID, chatID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ToggleLike", reflect.TypeOf((*MockChatDao)(nil).ToggleLike), userID, chatID)
}
