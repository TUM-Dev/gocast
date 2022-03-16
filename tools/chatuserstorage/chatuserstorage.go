package chatuserstorage

import (
	"TUM-Live/model"
	"sync"
)

type UserStorage interface {
	GetAll() []*model.User
	Add(user *model.User)
	Remove(user *model.User)
}

type ChatUserStorage struct {
	mtx  sync.RWMutex
	uMap map[uint]*model.User
}

func InitChatUserStorage() UserStorage {
	return &ChatUserStorage{uMap: make(map[uint]*model.User)}
}

func (c *ChatUserStorage) GetAll() []*model.User {
	var users []*model.User
	for _, u := range c.uMap {
		users = append(users, u)
	}
	return users
}

func (c *ChatUserStorage) Add(user *model.User) {
	c.mtx.Lock()
	if user != nil {
		c.uMap[user.ID] = user
	}
	c.mtx.Unlock()
}

func (c *ChatUserStorage) Remove(user *model.User) {
	c.mtx.Lock()
	if user != nil {
		delete(c.uMap, user.ID)
	}
	c.mtx.Unlock()
}
