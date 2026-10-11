package api

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

type fakeViewers map[uint]int

func (f fakeViewers) Viewers(id uint) int { return f[id] }

func TestCollectStats(t *testing.T) {
	// collect runs one tick and returns the rows it wrote, by stream.
	collect := func(t *testing.T, live []model.Stream, v2 ViewerCounter) map[uint]model.Stat {
		ctrl := gomock.NewController(t)
		streams := mock_dao.NewMockStreamsDao(ctrl)
		streams.EXPECT().GetCurrentLive(gomock.Any()).Return(live, nil).AnyTimes()
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

	t.Run("a live stream gets a row", func(t *testing.T) {
		got := collect(t, live(9101), fakeViewers{9101: 3})
		if got[9101].Viewers != 3 || !got[9101].Live {
			t.Errorf("stat = %+v, want 3 live viewers", got[9101])
		}
	})

	t.Run("streams that are not live or have no viewers get no row", func(t *testing.T) {
		got := collect(t, live(9105), fakeViewers{9106: 4})
		if len(got) != 0 {
			t.Errorf("rows = %+v, want none", got)
		}
	})
}
