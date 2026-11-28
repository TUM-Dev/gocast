package dao

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

func setupStatsTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:stats_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// The shared in-memory database lives as long as a connection does; -count=N
	// would otherwise add to the previous run's rows.
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.Stat{}))
	DB = db
}

func TestAddVodViewCountsPerStream(t *testing.T) {
	setupStatsTestDB(t)
	streams, stats := NewStreamsDao(), NewStatisticsDao()

	require.NoError(t, streams.AddVodView("1"))
	require.NoError(t, streams.AddVodView("2"))
	require.NoError(t, streams.AddVodView("2"))
	require.NoError(t, streams.AddVodView("2"))

	for id, want := range map[uint]int{1: 1, 2: 3} {
		got, err := stats.GetLectureNumVodViews(id)
		require.NoError(t, err)
		assert.Equal(t, want, got, "stream %d", id)
	}
}

func TestAddVodViewLosesNoConcurrentViews(t *testing.T) {
	setupStatsTestDB(t)
	// SQLite locks the table on concurrent writers. One connection still lets the
	// goroutines interleave between statements, which is where a read-then-save lost
	// views.
	sqlDB, err := DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	streams := NewStreamsDao()
	// The first view creates the row; the rest race on the increment.
	require.NoError(t, streams.AddVodView("1"))

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, streams.AddVodView("1"))
		}()
	}
	wg.Wait()

	got, err := NewStatisticsDao().GetLectureNumVodViews(1)
	require.NoError(t, err)
	assert.Equal(t, 21, got)
}

func TestLectureNumLiveViewsIgnoresVodRows(t *testing.T) {
	setupStatsTestDB(t)
	now := time.Now()
	require.NoError(t, DB.Create(&[]model.Stat{
		{Time: now, StreamID: 1, Viewers: 12, Live: true},
		{Time: now, StreamID: 1, Viewers: 300, Live: false},
	}).Error)

	got, err := NewStatisticsDao().GetLectureNumLiveViews(1)
	require.NoError(t, err)
	assert.Equal(t, 12, got)
}

func TestStatsSkipSoftDeletedRows(t *testing.T) {
	setupStatsTestDB(t)
	now := time.Now()
	require.NoError(t, DB.Create(&[]model.Stat{
		{Time: now, StreamID: 1, Viewers: 5, Live: false},
		{Time: now, StreamID: 1, Viewers: 7, Live: true},
	}).Error)
	require.NoError(t, DB.Where("stream_id = ?", 1).Delete(&model.Stat{}).Error)
	stats := NewStatisticsDao()

	vod, err := stats.GetLectureNumVodViews(1)
	require.NoError(t, err)
	assert.Zero(t, vod)
	live, err := stats.GetLectureNumLiveViews(1)
	require.NoError(t, err)
	assert.Zero(t, live)
}
