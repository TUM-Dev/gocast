package dao

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=offline_package.go -destination ../mock_dao/offline_package.go

type OfflinePackageDao interface {
	Create(ctx context.Context, p *model.OfflinePackage) error
	Get(ctx context.Context, id uint) (model.OfflinePackage, error)
	// ListForUser returns the user's packages with the given ids; ids of other users'
	// packages are left out.
	ListForUser(ctx context.Context, userID uint, ids []uint) ([]model.OfflinePackage, error)
	Update(ctx context.Context, p *model.OfflinePackage) error
	// CountSince counts the packages the user asked for since t.
	CountSince(ctx context.Context, userID uint, t time.Time) (int64, error)
	// CountActive counts preparing and ready packages, of one user or, with userID 0,
	// of everybody: these are the ones that take up space.
	CountActive(ctx context.Context, userID uint) (int64, error)
	// FailStalePreparing marks packages preparing since before t as failed, and returns
	// how many it marked.
	FailStalePreparing(ctx context.Context, t time.Time, reason string) (int64, error)
	// ExpireStaleReady marks ready packages made before t as failed: their downloads
	// were abandoned, and the runners have removed or are about to remove them.
	ExpireStaleReady(ctx context.Context, t time.Time, reason string) (int64, error)
}

type offlinePackageDao struct {
	db *gorm.DB
}

func NewOfflinePackageDao() OfflinePackageDao {
	return offlinePackageDao{db: DB}
}

func (d offlinePackageDao) Create(ctx context.Context, p *model.OfflinePackage) error {
	return d.db.WithContext(ctx).Create(p).Error
}

func (d offlinePackageDao) Get(ctx context.Context, id uint) (p model.OfflinePackage, err error) {
	err = d.db.WithContext(ctx).First(&p, id).Error
	return p, err
}

func (d offlinePackageDao) ListForUser(ctx context.Context, userID uint, ids []uint) (ps []model.OfflinePackage, err error) {
	if len(ids) == 0 {
		return nil, nil
	}
	err = d.db.WithContext(ctx).Where("user_id = ? AND id IN ?", userID, ids).Find(&ps).Error
	return ps, err
}

func (d offlinePackageDao) Update(ctx context.Context, p *model.OfflinePackage) error {
	return d.db.WithContext(ctx).Save(p).Error
}

func (d offlinePackageDao) CountSince(ctx context.Context, userID uint, t time.Time) (n int64, err error) {
	err = d.db.WithContext(ctx).Model(&model.OfflinePackage{}).
		Where("user_id = ? AND created_at >= ?", userID, t).Count(&n).Error
	return n, err
}

func (d offlinePackageDao) CountActive(ctx context.Context, userID uint) (n int64, err error) {
	q := d.db.WithContext(ctx).Model(&model.OfflinePackage{}).
		Where("state IN ?", []model.OfflinePackageState{model.OfflinePreparing, model.OfflineReady})
	if userID != 0 {
		q = q.Where("user_id = ?", userID)
	}
	err = q.Count(&n).Error
	return n, err
}

func (d offlinePackageDao) FailStalePreparing(ctx context.Context, t time.Time, reason string) (int64, error) {
	res := d.db.WithContext(ctx).Model(&model.OfflinePackage{}).
		Where("state = ? AND created_at < ?", model.OfflinePreparing, t).
		Updates(map[string]any{"state": model.OfflineFailed, "error": reason})
	return res.RowsAffected, res.Error
}

func (d offlinePackageDao) ExpireStaleReady(ctx context.Context, t time.Time, reason string) (int64, error) {
	res := d.db.WithContext(ctx).Model(&model.OfflinePackage{}).
		Where("state = ? AND created_at < ?", model.OfflineReady, t).
		Updates(map[string]any{"state": model.OfflineFailed, "error": reason})
	return res.RowsAffected, res.Error
}
