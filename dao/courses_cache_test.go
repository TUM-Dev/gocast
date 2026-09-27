package dao

import (
	"context"
	"testing"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

func setupCourseCacheTestDB(t *testing.T) {
	t.Helper()

	cache, err := ristretto.NewCache[string, any](&ristretto.Config[string, any]{
		NumCounters: 1e7,
		MaxCost:     1 << 30,
		BufferItems: 64,
	})
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}
	Cache = cache

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.Course{}); err != nil {
		t.Fatalf("migrate course model: %v", err)
	}
	DB = db
}

func TestCourseCacheInvalidationOnDeleteAndUndelete(t *testing.T) {
	setupCourseCacheTestDB(t)

	course := model.Course{
		Model:        gorm.Model{ID: 1},
		UserID:       1,
		Name:         "Test Course",
		Slug:         "test-course",
		Year:         2026,
		TeachingTerm: "S",
	}
	if err := DB.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}

	dao := CoursesDaoImpl{db: DB}

	Cache.SetWithTTL("publicCourses2026S", []model.Course{course}, 1, time.Minute)
	Cache.Wait()
	if _, found := Cache.Get("publicCourses2026S"); !found {
		t.Fatal("test setup did not populate cache")
	}

	dao.DeleteCourse(course)
	if _, found := Cache.Get("publicCourses2026S"); found {
		t.Fatal("DeleteCourse did not clear the course cache")
	}

	if err := dao.UnDeleteCourse(context.Background(), course); err != nil {
		t.Fatalf("undelete course: %v", err)
	}
	Cache.SetWithTTL("publicCourses2026S", []model.Course{course}, 1, time.Minute)
	Cache.Wait()
	if _, found := Cache.Get("publicCourses2026S"); !found {
		t.Fatal("test setup did not repopulate cache for undelete check")
	}

	if err := dao.UnDeleteCourse(context.Background(), course); err != nil {
		t.Fatalf("second undelete course: %v", err)
	}
	if _, found := Cache.Get("publicCourses2026S"); found {
		t.Fatal("UnDeleteCourse did not clear the course cache")
	}
}
