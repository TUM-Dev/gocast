package dao

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

type ActionDao interface {
	CreateAction(ctx context.Context, action *model.Action) error
	CompleteAction(ctx context.Context, actionID string) error
	GetActionByID(ctx context.Context, actionID string) (model.Action, error)
	GetActionsByJobID(ctx context.Context, jobID string) ([]model.Action, error)
	GetAwaitingActions(ctx context.Context) ([]model.Action, error)
	GetRunningActions(ctx context.Context) ([]model.Action, error)
	GetAll(ctx context.Context) ([]model.Action, error)
	GetAllFailedActions(ctx context.Context) ([]model.Action, error)
}

type actionDao struct {
	db *gorm.DB
}

func NewActionDao() ActionDao {
	return actionDao{db: DB}
}

func (d actionDao) CreateAction(ctx context.Context, action *model.Action) error {
	return d.db.WithContext(ctx).Create(&action).Error
}

func (d actionDao) CompleteAction(ctx context.Context, actionID string) error {
	return d.db.WithContext(ctx).Model(&model.Action{}).Where("id = ?", actionID).Update("status", "completed").Error
}

func (d actionDao) GetActionByID(ctx context.Context, actionID string) (model.Action, error) {
	var action model.Action
	err := d.db.WithContext(ctx).First(&action, "id = ?", actionID).Error
	return action, err
}

func (d actionDao) GetActionsByJobID(ctx context.Context, jobID string) ([]model.Action, error) {
	var actions []model.Action
	err := d.db.WithContext(ctx).Find(&actions, "job_id = ?", jobID).Error
	return actions, err
}

func (d actionDao) GetAwaitingActions(ctx context.Context) ([]model.Action, error) {
	var actions []model.Action
	err := d.db.WithContext(ctx).Find(&actions, "status = ?", 3).Error
	return actions, err
}

func (d actionDao) GetRunningActions(ctx context.Context) ([]model.Action, error) {
	var actions []model.Action
	err := d.db.WithContext(ctx).Find(&actions, "status = ?", 1).Error
	return actions, err
}

func (d actionDao) GetAll(ctx context.Context) ([]model.Action, error) {
	var actions []model.Action
	err := d.db.WithContext(ctx).Find(&actions).Error
	return actions, err
}

func (d actionDao) GetAllFailedActions(ctx context.Context) ([]model.Action, error) {
	var actions []model.Action
	err := d.db.WithContext(ctx).Find(&actions, "status = ?", 2).Error
	return actions, err
}
