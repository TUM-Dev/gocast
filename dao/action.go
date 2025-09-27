package dao

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=action.go -destination ../mock_dao/action.go

type ActionDao interface {
	// Get Action by ID
	Get(context.Context, uint) (model.Action, error)

	// Create a new Action for the database
	Create(context.Context, *model.Action) error

	// Delete a Action by id.
	Delete(context.Context, uint) error
}

type actionDao struct {
	db *gorm.DB
}

func NewActionDao() ActionDao {
	return actionDao{db: DB}
}

// Get a Action by id.
func (d actionDao) Get(c context.Context, id uint) (res model.Action, err error) {
	return res, d.db.WithContext(c).First(&res, id).Error
}

// Create a Action.
func (d actionDao) Create(c context.Context, it *model.Action) error {
	return d.db.WithContext(c).Create(it).Error
}

// Delete a Action by id.
func (d actionDao) Delete(c context.Context, id uint) error {
	return d.db.WithContext(c).Delete(&model.Action{}, id).Error
}
