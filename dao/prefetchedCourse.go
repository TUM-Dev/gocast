package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=prefetchedCourse.go -destination ../mock_dao/prefetchedCourse.go

type PrefetchedCourseDao interface {
	// Get PrefetchedCourse by ID
	Get(context.Context, uint) (model.PrefetchedCourse, error)

	// Create a new PrefetchedCourse for the database
	Create(context.Context, ...*model.PrefetchedCourse) error

	// Delete a PrefetchedCourse by id.
	Delete(context.Context, uint) error

	// Search for a PrefetchedCourse by name using fulltext index
	Search(context.Context, string) ([]model.PrefetchedCourse, error)
}

type prefetchedCourseDao struct {
	db *gorm.DB
}

func NewPrefetchedCourseDao() PrefetchedCourseDao {
	return prefetchedCourseDao{db: DB}
}

// Get a PrefetchedCourse by id.
func (d prefetchedCourseDao) Get(c context.Context, id uint) (res model.PrefetchedCourse, err error) {
	return res, DB.WithContext(c).First(&res, id).Error
}

// Create a PrefetchedCourse.
func (d prefetchedCourseDao) Create(c context.Context, it ...*model.PrefetchedCourse) error {
	return DB.WithContext(c).Clauses(clause.OnConflict{DoNothing: true}).Create(it).Error
}

// Delete a PrefetchedCourse by id.
func (d prefetchedCourseDao) Delete(c context.Context, id uint) error {
	return DB.WithContext(c).Delete(&model.PrefetchedCourse{}, id).Error
}

// Search for a PrefetchedCourse by name using fulltext index
func (d prefetchedCourseDao) Search(ctx context.Context, s string) ([]model.PrefetchedCourse, error) {
	var res = make([]model.PrefetchedCourse, 0)
	err := DB.WithContext(ctx).
		Raw("WITH tmp as (SELECT *, MATCH (name) AGAINST(? IN NATURAL LANGUAGE MODE) AS m "+
			"FROM "+(&model.PrefetchedCourse{}).TableName()+
			") SELECT * FROM tmp WHERE m>0 ORDER BY m DESC LIMIT 10", s).
		Scan(&res).
		Error
	return res, err
}
