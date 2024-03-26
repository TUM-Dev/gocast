package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

type ActionDao interface {
	CreateAction(action model.Action) error
	CompleteAction(actionID string) error
}

type actionDao struct {
	db *gorm.DB
}
