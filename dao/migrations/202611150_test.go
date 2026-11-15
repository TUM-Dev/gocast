package migrations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

func TestMigrate202611150HidesOnlyVodStats(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Stat{}))
	now := time.Now()
	require.NoError(t, db.Create(&[]model.Stat{
		{Time: now, StreamID: 1, Viewers: 40, Live: false},
		{Time: now, StreamID: 1, Viewers: 9, Live: true},
	}).Error)

	require.NoError(t, Migrate202611150().Migrate(db))

	var visible []model.Stat
	require.NoError(t, db.Find(&visible).Error)
	require.Len(t, visible, 1)
	assert.True(t, visible[0].Live)

	require.NoError(t, Migrate202611150().Rollback(db))
	var count int64
	require.NoError(t, db.Model(&model.Stat{}).Count(&count).Error)
	assert.EqualValues(t, 2, count)
}
