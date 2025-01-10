package dao

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=action.go -destination ../mock_dao/action.go

type ActionDao interface {
	CreateAction(ctx context.Context, action *model.Action) error
	CompleteAction(ctx context.Context, actionID uint) error
	GetActionByID(ctx context.Context, actionID uint) (model.Action, error)
	GetActionsByJobID(ctx context.Context, jobID uint) ([]model.Action, error)
	GetAwaitingActions(ctx context.Context) ([]model.Action, error)
	GetRunningActions(ctx context.Context) ([]model.Action, error)
	GetAll(ctx context.Context) ([]model.Action, error)
	GetAllFailedActions(ctx context.Context) ([]model.Action, error)
	UpdateAction(ctx context.Context, action *model.Action) error
	AssignRunner(ctx context.Context, action *model.Action, runner *model.Runner) error
	GetAllActionOfRunner(ctx context.Context, runnerID uint) ([]model.Action, error)
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

func (d actionDao) CompleteAction(ctx context.Context, actionID uint) error {
	return d.db.WithContext(ctx).Model(&model.Action{}).Where("id = ?", actionID).Update("status", "completed").Error
}

func (d actionDao) GetActionByID(ctx context.Context, actionID uint) (model.Action, error) {
	var action model.Action
	err := d.db.WithContext(ctx).First(&action, "id = ?", actionID).Error
	return action, err
}

func (d actionDao) GetActionsByJobID(ctx context.Context, jobID uint) ([]model.Action, error) {
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
	err := d.db.WithContext(ctx).Preload("AllRunners").Find(&actions, "status = ?", 1).Error
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

func (d actionDao) UpdateAction(ctx context.Context, action *model.Action) error {
	err := d.db.WithContext(ctx).Model(&model.Action{}).Where("id = ?", action.ID).Updates(action).Error
	act, _ := d.GetActionByID(ctx, action.ID)
	logger.Info("updated action", "action", len(act.AllRunners))
	return err
}

func (d actionDao) AssignRunner(ctx context.Context, action *model.Action, runner *model.Runner) error {
	return d.db.WithContext(ctx).Model(&action).Association("AllRunners").Append(runner)
}

func (d actionDao) GetAllActionOfRunner(ctx context.Context, runnerID uint) ([]model.Action, error) {
	var actions []model.Action
	err := d.db.WithContext(ctx).Joins("AllRunners").Where("id = ?", runnerID).Find(&actions).Error
	return actions, err
}
