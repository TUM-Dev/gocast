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
	revoked, err := d.RevokeIntegrationGrantForIntegration(ctx, grantID, 2)
	require.NoError(t, err)
	require.Zero(t, revoked.ID)
	_, err = d.GetIntegrationGrantCourse(ctx, grantID, 1)
	require.NoError(t, err)
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
	revoked, err = d.RevokeIntegrationGrantForIntegration(ctx, newID, 1)
	require.NoError(t, err)
	require.Equal(t, uint(2), revoked.ID)
	require.Equal(t, "winter", revoked.Slug)
	revoked, err = d.RevokeIntegrationGrantForIntegration(ctx, newID, 1)
	require.NoError(t, err)
	require.Zero(t, revoked.ID)
	grants, err = d.GetCourseIntegrationGrants(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, grants)
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

func TestIntegrationGrantStreamIsScopedToVisibleLecturesOfAnActiveGrant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Course{}, &model.Integration{}, &model.IntegrationGrant{}, &model.IntegrationRecording{}))
	// SQLite cannot build the MySQL FULLTEXT indexes of model.Stream. gorm creates them
	// after the table, so the full table is there and the query reads every column.
	if err := db.Migrator().CreateTable(&model.Stream{}); err != nil {
		require.True(t, db.Migrator().HasTable(&model.Stream{}), err)
	}
	require.NoError(t, db.Create(&[]model.Course{
		{Model: gorm.Model{ID: 1}, Name: "Granted", Slug: "granted"},
		{Model: gorm.Model{ID: 2}, Name: "Other", Slug: "other"},
	}).Error)
	require.NoError(t, db.Create(&[]model.Integration{{ID: 1, Name: "Course portal"}, {ID: 2, Name: "Other portal"}}).Error)
	revokedAt := time.Now()
	require.NoError(t, db.Create(&[]model.IntegrationGrant{
		{ID: 10, IntegrationID: 1, CourseID: 1},
		{ID: 11, IntegrationID: 1, CourseID: 1, RevokedAt: &revokedAt},
		{ID: 12, IntegrationID: 2, CourseID: 2},
	}).Error)
	require.NoError(t, db.Create(&[]model.Stream{
		{Model: gorm.Model{ID: 100}, CourseID: 1, Recording: true, PlaylistUrl: "https://edge/comb.m3u8"},
		{Model: gorm.Model{ID: 101}, CourseID: 1, Private: true},
		{Model: gorm.Model{ID: 102}, CourseID: 2},
		{Model: gorm.Model{ID: 103}, CourseID: 1},
	}).Error)
	require.NoError(t, db.Delete(&model.Stream{}, 103).Error)
	d := integrationGrantDao{db: db}
	ctx := context.Background()

	stream, err := d.GetIntegrationGrantStream(ctx, 10, 1, 100)
	require.NoError(t, err)
	require.Equal(t, uint(100), stream.ID)
	require.Equal(t, uint(1), stream.CourseID)
	require.Equal(t, "https://edge/comb.m3u8", stream.PlaylistUrl)

	for name, ids := range map[string][3]uint{
		"private lecture":         {10, 1, 101},
		"lecture of other course": {10, 1, 102},
		"deleted lecture":         {10, 1, 103},
		"revoked grant":           {11, 1, 100},
		"grant of other app":      {10, 2, 100},
		"other app's own grant":   {12, 1, 102},
	} {
		_, err := d.GetIntegrationGrantStream(ctx, ids[0], ids[1], ids[2])
		require.ErrorIs(t, err, gorm.ErrRecordNotFound, name)
	}

	require.NoError(t, db.Delete(&model.Course{}, 1).Error)
	_, err = d.GetIntegrationGrantStream(ctx, 10, 1, 100)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "lecture of deleted course")
}

func TestIntegrationUploadOwnership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Course{}, &model.Integration{}, &model.IntegrationGrant{}, &model.IntegrationRecording{}))
	// See TestIntegrationGrantStreamIsScopedToVisibleLecturesOfAnActiveGrant.
	if err := db.Migrator().CreateTable(&model.Stream{}); err != nil {
		require.True(t, db.Migrator().HasTable(&model.Stream{}), err)
	}
	require.NoError(t, db.Create(&[]model.Course{{Model: gorm.Model{ID: 1}, Name: "One", Slug: "one"}, {Model: gorm.Model{ID: 2}, Name: "Two", Slug: "two"}, {Model: gorm.Model{ID: 3}, Name: "Three", Slug: "three"}}).Error)
	require.NoError(t, db.Create(&model.Integration{ID: 1, Name: "Course portal"}).Error)
	revokedAt := time.Now()
	require.NoError(t, db.Create(&[]model.IntegrationGrant{
		{ID: 10, IntegrationID: 1, CourseID: 1}, {ID: 11, IntegrationID: 1, CourseID: 1, RevokedAt: &revokedAt},
		{ID: 12, IntegrationID: 1, CourseID: 2}, {ID: 13, IntegrationID: 1, CourseID: 3},
	}).Error)
	d := integrationGrantDao{db: db}
	ctx := context.Background()
	expires := time.Now().Add(10 * time.Minute)
	hash := func(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }
	ticketOf := func(ref string) []byte {
		var rec model.IntegrationRecording
		require.NoError(t, db.Where("external_ref = ?", ref).First(&rec).Error)
		return rec.TicketHash
	}

	stream, created, err := d.CreateIntegrationUpload(ctx, 1, 10, "reference-a-0000", model.Stream{Name: "Upload A"}, hash(1), expires)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, uint(1), stream.CourseID)
	require.Equal(t, hash(1), ticketOf("reference-a-0000"))

	again, created, err := d.CreateIntegrationUpload(ctx, 1, 10, "reference-a-0000", model.Stream{Name: "ignored"}, hash(2), expires)
	require.NoError(t, err)
	require.False(t, created, "a known reference keeps its lecture")
	require.Equal(t, stream.ID, again.ID)
	require.Equal(t, hash(2), ticketOf("reference-a-0000"), "the new ticket replaces the old one")
	var lectures int64
	require.NoError(t, db.Model(&model.Stream{}).Count(&lectures).Error)
	require.Equal(t, int64(1), lectures)

	_, _, err = d.CreateIntegrationUpload(ctx, 1, 12, "reference-a-0000", model.Stream{}, hash(3), expires)
	require.ErrorIs(t, err, ErrIntegrationUploadRefused, "a reference stays with its grant")
	_, _, err = d.CreateIntegrationUpload(ctx, 1, 11, "reference-b-0000", model.Stream{}, hash(3), expires)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "revoked grant")
	_, _, err = d.CreateIntegrationUpload(ctx, 2, 10, "reference-b-0000", model.Stream{}, hash(3), expires)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "another integration's grant")
	require.NoError(t, db.Delete(&model.Course{}, 3).Error)
	_, _, err = d.CreateIntegrationUpload(ctx, 1, 13, "reference-b-0000", model.Stream{}, hash(3), expires)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "deleted course")

	for field, ref := range map[string]string{"upload_started": "reference-c-0000", "closed": "reference-d-0000", "deleted": "reference-e-0000"} {
		created, _, err := d.CreateIntegrationUpload(ctx, 1, 10, ref, model.Stream{}, hash(byte(len(field))), expires)
		require.NoError(t, err)
		if field == "deleted" {
			require.NoError(t, db.Delete(&model.Stream{}, created.ID).Error)
		} else {
			require.NoError(t, db.Model(&model.IntegrationRecording{}).Where("external_ref = ?", ref).Update(field, true).Error)
		}
		_, _, err = d.CreateIntegrationUpload(ctx, 1, 10, ref, model.Stream{}, hash(9), expires)
		require.ErrorIs(t, err, ErrIntegrationUploadRefused, field)
	}

	// GoCast makes the first VOD of a course with private VODs private; the uploader
	// still reads its own lecture, until it closes the reference.
	require.NoError(t, db.Model(&model.Stream{}).Where("id = ?", stream.ID).Update("private", true).Error)
	_, err = d.GetIntegrationGrantStream(ctx, 10, 1, stream.ID)
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.IntegrationRecording{}).Where("external_ref = ?", "reference-a-0000").Update("closed", true).Error)
	_, err = d.GetIntegrationGrantStream(ctx, 10, 1, stream.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
