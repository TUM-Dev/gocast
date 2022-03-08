package chatuserstorage

import (
	"TUM-Live/model"
	"sync"
)

type User struct {
	Id uint `json:"id"`
	Name string `json:"name"`
}

type UserStorage interface {
	GetAll() []User
	Add(user *model.User)
	Remove(user *model.User)
}

type ChatUserStorage struct {
	mtx  sync.RWMutex
	uMap map[uint]string
}

func InitChatUserStorage() UserStorage {
	return &ChatUserStorage{uMap: make(map[uint]string)}
}

func (c *ChatUserStorage) GetAll() []User {
	var users []User
	for id, name := range c.uMap {
		users = append(users, User{id, name})
	}
	return users
}

func (c *ChatUserStorage) Add(user *model.User) {
	c.mtx.Lock()
	if user != nil {
		c.uMap[user.ID] = user.Name
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
