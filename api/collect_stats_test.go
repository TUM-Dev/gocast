package api

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools/realtime"
)

type fakeViewers map[uint]int

func (f fakeViewers) Viewers(id uint) int { return f[id] }

// v1Sessions registers n v1 sessions on the stream for the test. The clients are not
// connected, so broadcasting skips them, but they count as sessions.
func v1Sessions(t *testing.T, streamID uint, n int) {
	t.Helper()
	wsMapLock.Lock()
	for i := 0; i < n; i++ {
		sessionsMap[streamID] = append(sessionsMap[streamID], &sessionWrapper{
			session: &realtime.Context{Client: &realtime.Client{Id: "test"}},
		})
	}
	wsMapLock.Unlock()
	t.Cleanup(func() {
		wsMapLock.Lock()
		delete(sessionsMap, streamID)
		wsMapLock.Unlock()
	})
}

func TestCollectStats(t *testing.T) {
	// collect runs one tick and returns the rows it wrote, by stream.
	collect := func(t *testing.T, live []model.Stream, v2 ViewerCounter) map[uint]model.Stat {
		ctrl := gomock.NewController(t)
		streams := mock_dao.NewMockStreamsDao(ctrl)
		streams.EXPECT().GetCurrentLive(gomock.Any()).Return(live, nil).AnyTimes()
		// BroadcastStats looks sessions' streams up; none of these are recordings.
		streams.EXPECT().GetStreamByID(gomock.Any(), gomock.Any()).Return(model.Stream{}, nil).AnyTimes()
		stats := mock_dao.NewMockStatisticsDao(ctrl)
		got := map[uint]model.Stat{}
		stats.EXPECT().AddStat(gomock.Any()).DoAndReturn(func(s model.Stat) error {
			got[s.StreamID] = s
			return nil
		}).AnyTimes()

		CollectStats(dao.DaoWrapper{StreamsDao: streams, StatisticsDao: stats}, v2)()
		return got
	}
	live := func(ids ...uint) []model.Stream {
		var out []model.Stream
		for _, id := range ids {
			s := model.Stream{LiveNow: true}
			s.ID = id
			out = append(out, s)
		}
		return out
	}

	t.Run("a live stream with only v2 sockets gets a row", func(t *testing.T) {
		got := collect(t, live(9101), fakeViewers{9101: 3})
		if got[9101].Viewers != 3 || !got[9101].Live {
			t.Errorf("stat = %+v, want 3 live viewers", got[9101])
		}
	})

	t.Run("v1 and v2 viewers are summed", func(t *testing.T) {
		v1Sessions(t, 9102, 2)
		got := collect(t, live(9102), fakeViewers{9102: 5})
		if got[9102].Viewers != 7 {
			t.Errorf("viewers = %d, want 7", got[9102].Viewers)
		}
	})

	t.Run("v1 only still works without a hub", func(t *testing.T) {
		v1Sessions(t, 9103, 2)
		got := collect(t, live(9103), nil)
		if got[9103].Viewers != 2 {
			t.Errorf("viewers = %d, want 2", got[9103].Viewers)
		}
	})

	t.Run("streams that are not live or have no viewers get no row", func(t *testing.T) {
		v1Sessions(t, 9104, 1) // watched, but not in the live list
		got := collect(t, live(9105), fakeViewers{9106: 4})
		if len(got) != 0 {
			t.Errorf("rows = %+v, want none", got)
		}
	})
}
