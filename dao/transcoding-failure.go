package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=transcoding-failure.go -destination ../mock_dao/transcoding-failure.go

type TranscodingFailureDao interface {
	//All returns all open transcoding failures
	All() ([]model.TranscodingFailure, error)

	//New creates a new transcoding failure
	New(*model.TranscodingFailure) error

	//Delete deletes a transcoding failure
	Delete(id uint) error
}

func NewTranscodingFailureDao() TranscodingFailureDao {
	return &transcodingFailureDao{db: DB}
}

type transcodingFailureDao struct {
	db *gorm.DB
}

// All returns all open transcoding failures
func (t transcodingFailureDao) All() (failures []model.TranscodingFailure, err error) {
	return failures, DB.Preload("Stream").Find(&failures).Error
}

// New creates a new transcoding failure
func (t transcodingFailureDao) New(failure *model.TranscodingFailure) error {
	return DB.Create(failure).Error
}

// Delete deletes a transcoding failure
func (t transcodingFailureDao) Delete(id uint) error {
	return DB.Delete(&model.TranscodingFailure{}, id).Error
}
