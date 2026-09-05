package dao

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

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
	codeHash := bytes.Repeat([]byte{1}, 32)
	_, _, err = d.RedeemIntegrationAuthorizationCode(ctx, 2, codeHash, stateHash, time.Now())
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, _, err = d.RedeemIntegrationAuthorizationCode(ctx, 1, codeHash, bytes.Repeat([]byte{9}, 32), time.Now())
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, _, err = d.RedeemIntegrationAuthorizationCode(ctx, 1, codeHash, stateHash, expiresAt)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	redeemedID, courseID, err := d.RedeemIntegrationAuthorizationCode(ctx, 1, codeHash, stateHash, time.Now())
	require.NoError(t, err)
	require.Equal(t, grantID, redeemedID)
	require.Equal(t, uint(2), courseID)
	_, _, err = d.RedeemIntegrationAuthorizationCode(ctx, 1, codeHash, stateHash, time.Now())
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	course, err := d.GetIntegrationGrantCourse(ctx, grantID, 1)
	require.NoError(t, err)
	require.Equal(t, uint(2), course.ID)
	_, err = d.GetIntegrationGrantCourse(ctx, grantID, 2)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	if err := d.RevokeIntegrationGrant(ctx, grantID, 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("revocation with wrong course: %v", err)
	}
	if err := d.RevokeIntegrationGrant(ctx, grantID, 2); err != nil {
		t.Fatal(err)
	}
	_, _, err = d.RedeemIntegrationAuthorizationCode(ctx, 1, bytes.Repeat([]byte{2}, 32), stateHash, time.Now())
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = d.GetIntegrationGrantCourse(ctx, grantID, 1)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	grants, err := d.GetCourseIntegrationGrants(ctx, 2)
	if err != nil || len(grants) != 0 {
		t.Fatalf("revoked grant still listed: %v, %v", grants, err)
	}
	newID, err := d.ApproveIntegrationCourse(ctx, 1, 2, bytes.Repeat([]byte{3}, 32), stateHash, expiresAt)
	if err != nil || newID == grantID {
		t.Fatalf("reapproval reused revoked grant: %d, %v", newID, err)
	}
	require.NoError(t, db.Delete(&model.Course{}, 2).Error)
	_, err = d.GetIntegrationGrantCourse(ctx, newID, 1)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRedeemIntegrationAuthorizationCodeIsAtomicAndRequiresLiveGrant(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	prefix := fmt.Sprintf("g04_%d_", time.Now().UnixNano())
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Integration{}, &model.IntegrationGrant{}, &model.IntegrationAuthorizationCode{}))
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&model.IntegrationAuthorizationCode{}, &model.IntegrationGrant{}, &model.Integration{})
	})
	require.NoError(t, db.Create(&model.Integration{ID: 7, Name: "Test integration", ReturnURL: "https://example.invalid/callback"}).Error)
	redeemer := integrationGrantDao{db: db}
	now := time.Now()
	seed := func(code byte, revoked bool) ([]byte, []byte) {
		grant := model.IntegrationGrant{IntegrationID: 7, CourseID: 456, CreatedAt: now}
		if revoked {
			grant.RevokedAt = &now
		}
		require.NoError(t, db.Create(&grant).Error)
		codeHash, stateHash := bytes.Repeat([]byte{code}, 32), bytes.Repeat([]byte{9}, 32)
		require.NoError(t, db.Create(&model.IntegrationAuthorizationCode{
			CodeHash: codeHash, StateHash: stateHash, IntegrationID: 7,
			GrantID: grant.ID, ExpiresAt: now.Add(time.Minute),
		}).Error)
		return codeHash, stateHash
	}

	codeHash, stateHash := seed(1, false)
	_, _, err = redeemer.RedeemIntegrationAuthorizationCode(context.Background(), 7, codeHash, bytes.Repeat([]byte{8}, 32), now)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	grantID, courseID, err := redeemer.RedeemIntegrationAuthorizationCode(context.Background(), 7, codeHash, stateHash, now)
	require.NoError(t, err)
	require.NotZero(t, grantID)
	require.Equal(t, uint(456), courseID)
	_, _, err = redeemer.RedeemIntegrationAuthorizationCode(context.Background(), 7, codeHash, stateHash, now)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	revokedCode, revokedState := seed(2, true)
	_, _, err = redeemer.RedeemIntegrationAuthorizationCode(context.Background(), 7, revokedCode, revokedState, now)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	concurrentCode, concurrentState := seed(3, false)
	start, results := make(chan struct{}), make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, _, redeemErr := redeemer.RedeemIntegrationAuthorizationCode(context.Background(), 7, concurrentCode, concurrentState, now)
			results <- redeemErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	var successes, missing int
	for result := range results {
		if result == nil {
			successes++
		} else if errors.Is(result, gorm.ErrRecordNotFound) {
			missing++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, missing)
}
