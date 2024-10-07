package dao

import (
	"context"

	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=worker.go -destination ../mock_dao/worker.go

type WorkerDao interface {
	CreateWorker(worker *model.Worker) error
	SaveWorker(worker model.Worker) error

	GetAllWorkers([]model.Organization) ([]model.Worker, error)
	GetAliveWorkers(uint) []model.Worker
	GetWorkerByHostname(ctx context.Context, address string, hostname string) (model.Worker, error)
	GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error)

	DeleteWorker(workerID string) error
}

type workerDao struct {
	db *gorm.DB
}

func NewWorkerDao() WorkerDao {
	return workerDao{db: DB}
}

func (d workerDao) CreateWorker(worker *model.Worker) error {
	return DB.Create(worker).Error
}

func (d workerDao) SaveWorker(worker model.Worker) error {
	return DB.Save(&worker).Error
}

// Return all workers for a user's administered organizations
// Return all workers for a user's administered organizations
func (d workerDao) GetAllWorkers(organizations []model.Organization) ([]model.Worker, error) {
	var workers []model.Worker
	err := DB.Where("organization_id IN (?)", getOrganizationIDs(organizations)).Find(&workers).Error
	return workers, err
}

// Helper function to extract organization IDs from a slice of organizations
func getOrganizationIDs(organizations []model.Organization) []uint {
	ids := make([]uint, len(organizations))
	for i, organization := range organizations {
		ids[i] = organization.ID
	}
	return ids
}

// GetAliveWorkers returns all workers that were active within the last 5 minutes
func (d workerDao) GetAliveWorkers(organizationID uint) []model.Worker { // TODO: @cb
	var workers []model.Worker

	rawSQL := `
WITH RECURSIVE organization_hierarchy AS (
	SELECT id, parent_id FROM organizations WHERE id = ?
	UNION ALL
	SELECT s.id, s.parent_id FROM organizations s
	INNER JOIN organization_hierarchy sh ON s.id = sh.parent_id
)
SELECT w.* FROM workers w
INNER JOIN organization_hierarchy sh ON sh.id = w.organization_id
WHERE w.last_seen > NOW() - INTERVAL 5 MINUTE OR w.shared = true
`
	DB.Raw(rawSQL, organizationID).Scan(&workers)
	return workers
}

func (d workerDao) GetWorkerByHostname(ctx context.Context, address string, hostname string) (model.Worker, error) {
	var worker model.Worker
	err := DB.Where("address = ? AND host = ?", address, hostname).First(&worker).Error
	return worker, err
}

func (d workerDao) GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error) {
	var worker model.Worker
	dbErr := DB.First(&worker, "worker_id = ?", workerID).Error
	return worker, dbErr
}

func (d workerDao) DeleteWorker(workerID string) error {
	return DB.Where("worker_id = ?", workerID).Delete(&model.Worker{}).Error
}
