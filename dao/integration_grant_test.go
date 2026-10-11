package dao

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

func TestIntegrationCourseGrants(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Course{}, &model.Integration{}, &model.IntegrationGrant{}, &model.IntegrationAuthorizationCode{}); err != nil {
		t.Fatal(err)
	}
	courses := []model.Course{
		{Model: gorm.Model{ID: 1}, UserID: 1, Name: "Summer course", Slug: "summer", Year: 2026, TeachingTerm: "S"},
		{Model: gorm.Model{ID: 2}, UserID: 2, Name: "Winter course", Slug: "winter", Year: 2026, TeachingTerm: "W"},
		{Model: gorm.Model{ID: 3}, UserID: 2, Name: "Other course", Slug: "other", Year: 2027, TeachingTerm: "W"},
	}
	if err := db.Create(&courses).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO course_admins (course_id, user_id) VALUES (1, 1), (2, 1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Integration{ID: 1, Name: "Course portal"}).Error; err != nil {
		t.Fatal(err)
	}
	d := integrationGrantDao{db: db}
	ctx := context.Background()
	eligible, err := d.GetAuthorizableCourses(ctx, 1)
	if err != nil || len(eligible) != 2 || eligible[0].ID != 2 || eligible[1].ID != 1 {
		t.Fatalf("eligible courses: %v, %v", eligible, err)
	}
	stateHash := bytes.Repeat([]byte{4}, 32)
	expiresAt := time.Now().Add(5 * time.Minute)
	grantID, err := d.ApproveIntegrationCourse(ctx, 1, 2, bytes.Repeat([]byte{1}, 32), stateHash, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	repeatedID, err := d.ApproveIntegrationCourse(ctx, 1, 2, bytes.Repeat([]byte{2}, 32), stateHash, expiresAt)
	if err != nil || repeatedID != grantID {
		t.Fatalf("reapproval did not reuse active grant: %d, %v", repeatedID, err)
	}
	if err := d.RevokeIntegrationGrant(ctx, grantID, 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("revocation with wrong course: %v", err)
	}
	if err := d.RevokeIntegrationGrant(ctx, grantID, 2); err != nil {
		t.Fatal(err)
	}
	grants, err := d.GetCourseIntegrationGrants(ctx, 2)
	if err != nil || len(grants) != 0 {
		t.Fatalf("revoked grant still listed: %v, %v", grants, err)
	}
	newID, err := d.ApproveIntegrationCourse(ctx, 1, 2, bytes.Repeat([]byte{3}, 32), stateHash, expiresAt)
	if err != nil || newID == grantID {
		t.Fatalf("reapproval reused revoked grant: %d, %v", newID, err)
	}
}
