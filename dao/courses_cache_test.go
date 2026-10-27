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

// cacheTestStream is the part of the streams table these tests touch. model.Stream
// cannot be migrated on sqlite (its FULLTEXT indexes), and UnDeleteCourse only reads
// and writes these columns.
type cacheTestStream struct {
	gorm.Model
	CourseID uint
	Name     string
}

func (cacheTestStream) TableName() string { return "streams" }

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
	if err := db.AutoMigrate(&model.Course{}, &cacheTestStream{}); err != nil {
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

func TestUnDeleteCourseRestoresTheStreamsDeletedWithIt(t *testing.T) {
	setupCourseCacheTestDB(t)
	course := model.Course{Model: gorm.Model{ID: 70}, UserID: 1, Name: "Restore Me", Slug: "restore-me", Year: 2026, TeachingTerm: "S"}
	if err := DB.Create(&course).Error; err != nil {
		t.Fatal(err)
	}
	earlier := cacheTestStream{Model: gorm.Model{ID: 701}, CourseID: 70, Name: "removed on its own"}
	with := cacheTestStream{Model: gorm.Model{ID: 702}, CourseID: 70, Name: "deleted with the course"}
	other := cacheTestStream{Model: gorm.Model{ID: 703}, CourseID: 71, Name: "another course"}
	for _, s := range []*cacheTestStream{&earlier, &with, &other} {
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	// One stream was removed months ago, on its own.
	if err := DB.Delete(&earlier).Error; err != nil {
		t.Fatal(err)
	}
	DB.Unscoped().Model(&model.Stream{}).Where("id = ?", 701).Update("deleted_at", time.Now().Add(-90*24*time.Hour))

	course.Streams = []model.Stream{{Model: gorm.Model{ID: 702}}}
	CoursesDaoImpl{db: DB}.DeleteCourse(course)
	if err := (CoursesDaoImpl{db: DB}).UnDeleteCourse(context.Background(), course); err != nil {
		t.Fatalf("UnDeleteCourse: %v", err)
	}

	var live []uint
	DB.Model(&cacheTestStream{}).Order("id").Pluck("id", &live)
	if len(live) != 2 || live[0] != 702 || live[1] != 703 {
		t.Fatalf("live streams = %v, want the one deleted with the course, and the other course's", live)
	}
	var c model.Course
	if err := DB.First(&c, 70).Error; err != nil {
		t.Fatalf("course not restored: %v", err)
	}
}
