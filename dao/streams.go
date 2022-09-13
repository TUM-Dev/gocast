package dao

import (
	"context"
	"fmt"
	"gorm.io/gorm/clause"
	"strconv"
	"time"

	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=streams.go -destination ../mock_dao/streams.go

type StreamsDao interface {
	CreateStream(stream *model.Stream) error
	AddVodView(id string) error

	GetDueStreamsForWorkers() []model.Stream
	GetDuePremieresForWorkers() []model.Stream
	GetStreamByKey(ctx context.Context, key string) (stream model.Stream, err error)
	GetUnitByID(id string) (model.StreamUnit, error)
	GetStreamByTumOnlineID(ctx context.Context, id uint) (stream model.Stream, err error)
	GetStreamsByIds(ids []uint) ([]model.Stream, error)
	GetStreamByID(ctx context.Context, id string) (stream model.Stream, err error)
	GetWorkersForStream(stream model.Stream) ([]model.Worker, error)
	GetAllStreams() ([]model.Stream, error)
	GetCurrentLive(ctx context.Context) (currentLive []model.Stream, err error)
	GetCurrentLiveNonHidden(ctx context.Context) (currentLive []model.Stream, err error)
	GetLiveStreamsInLectureHall(lectureHallId uint) ([]model.Stream, error)
	GetStreamsWithWatchState(courseID uint, userID uint) (streams []model.Stream, err error)

	SetLectureHall(streamIDs []uint, lectureHallID uint) error
	UnsetLectureHall(streamIDs []uint) error
	UpdateStream(stream model.Stream) error
	SaveWorkerForStream(stream model.Stream, worker model.Worker) error
	ClearWorkersForStream(stream model.Stream) error
	UpdateSilences(silences []model.Silence, streamID string) error
	DeleteSilences(streamID string) error
	UpdateStreamFullAssoc(vod *model.Stream) error
	SetStreamNotLiveById(streamID uint) error
	SetStreamLiveNowTimestampById(streamID uint, liveNowTimestamp time.Time) error
	SavePauseState(streamID uint, paused bool) error
	SaveEndedState(streamID uint, hasEnded bool) error
	SaveCOMBURL(stream *model.Stream, url string)
	SaveCAMURL(stream *model.Stream, url string)
	SavePRESURL(stream *model.Stream, url string)
	SaveTranscodingProgress(progress model.TranscodingProgress) error
	RemoveTranscodingProgress(streamVersion model.StreamVersion, streamId uint) error
	GetTranscodingProgressByVersion(streamVersion model.StreamVersion, streamId uint) (model.TranscodingProgress, error)
	SaveStream(vod *model.Stream) error
	ToggleVisibility(streamId uint, private bool) error

	DeleteStream(streamID string)
	DeleteUnit(id uint)
	DeleteStreamsWithTumID(ids []uint)
	UpdateLectureSeries(model.Stream) error
	DeleteLectureSeries(string) error
}

type streamsDao struct {
	db *gorm.DB
}

func (d streamsDao) GetTranscodingProgressByVersion(v model.StreamVersion, streamId uint) (p model.TranscodingProgress, err error) {
	err = DB.Where("version = ? AND stream_id = ?", v, streamId).First(&p).Error
	return
}

func NewStreamsDao() StreamsDao {
	return streamsDao{db: DB}
}

func (d streamsDao) CreateStream(stream *model.Stream) error {
	return DB.Create(stream).Error
}

func (d streamsDao) SaveTranscodingProgress(progress model.TranscodingProgress) error {
	return DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&progress).Error
}

// AddVodView Adds a stat entry to the database or increases the one existing for this hour
func (d streamsDao) AddVodView(id string) error {
	intId, err := strconv.Atoi(id)
	if err != nil {
		return err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		t := time.Now()
		tFrom := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.Local)
		tUntil := tFrom.Add(time.Hour)
		var stat *model.Stat
		err := DB.First(&stat, "live = 0 AND time BETWEEN ? and ?", tFrom, tUntil).Error
		if err != nil { // first view this hour, create
			stat := model.Stat{
				Time:     tFrom,
				StreamID: uint(intId),
				Viewers:  1,
				Live:     false,
			}
			err = tx.Create(&stat).Error
			return err
		} else {
			stat.Viewers += 1
			err = tx.Save(&stat).Error
			return err
		}
	})
	return err
}

// GetDueStreamsForWorkers retrieves all streams that due to be streamed in a lecture hall.
func (d streamsDao) GetDueStreamsForWorkers() []model.Stream {
	var res []model.Stream
	DB.Model(&model.Stream{}).
		Where("lecture_hall_id IS NOT NULL AND start BETWEEN NOW() AND DATE_ADD(NOW(), INTERVAL 10 MINUTE)" +
			"AND live_now = false AND recording = false AND (ended = false OR ended IS NULL)").
		Scan(&res)
	return res
}

func (d streamsDao) GetDuePremieresForWorkers() []model.Stream {
	var res []model.Stream
	DB.Preload("Files").
		Find(&res, "premiere AND start BETWEEN DATE_SUB(NOW(), INTERVAL 10 MINUTE) AND DATE_ADD(NOW(), INTERVAL 5 SECOND) AND live_now = false AND recording = false")
	return res
}

func (d streamsDao) GetStreamByKey(ctx context.Context, key string) (stream model.Stream, err error) {
	var res model.Stream
	err = DB.First(&res, "stream_key = ?", key).Error
	return res, err
}

func (d streamsDao) GetUnitByID(id string) (model.StreamUnit, error) {
	var unit model.StreamUnit
	err := DB.First(&unit, "id = ?", id).Error
	return unit, err
}

func (d streamsDao) GetStreamByTumOnlineID(ctx context.Context, id uint) (stream model.Stream, err error) {
	var res model.Stream
	err = DB.Preload("Chats").First(&res, "tum_online_event_id = ?", id).Error
	if err != nil {
		return res, err
	}
	return res, nil
}

// GetStreamsByIds get multiple streams by their ids
func (d streamsDao) GetStreamsByIds(ids []uint) ([]model.Stream, error) {
	var streams []model.Stream
	err := DB.Find(&streams, ids).Error
	return streams, err
}

func (d streamsDao) GetStreamByID(ctx context.Context, id string) (stream model.Stream, err error) {
	if cached, found := Cache.Get(fmt.Sprintf("streambyid%v", id)); found {
		return cached.(model.Stream), nil
	}
	var res model.Stream
	err = DB.
		Preload("VideoSections", func(db *gorm.DB) *gorm.DB {
			return db.Order("start_hours, start_minutes, start_seconds asc")
		}).
		Preload("Files").
		Preload("Silences").
		Preload("Units", func(db *gorm.DB) *gorm.DB {
			return db.Order("unit_start asc")
		}).First(&res, "id = ?", id).Error
	if err != nil {
		fmt.Printf("error getting stream by id: %v\n", err)
		return res, err
	}
	Cache.SetWithTTL(fmt.Sprintf("streambyid%v", id), res, 1, time.Second*10)
	return res, nil
}

func (d streamsDao) UpdateLectureSeries(stream model.Stream) error {
	defer Cache.Clear()
	err := DB.Table("streams").Where(
		"`series_identifier` = ? AND `deleted_at` IS NULL",
		stream.SeriesIdentifier,
	).Updates(map[string]interface{}{
		"name":        stream.Name,
		"description": stream.Description,
	}).Error
	return err
}

func (d streamsDao) DeleteLectureSeries(seriesIdentifier string) error {
	defer Cache.Clear()
	err := DB.Delete(&model.Stream{}, "`series_identifier` = ?", seriesIdentifier).Error
	return err
}

// GetWorkersForStream retrieves all workers for a given stream with streamID
func (d streamsDao) GetWorkersForStream(stream model.Stream) ([]model.Worker, error) {
	var res []model.Worker
	err := DB.Preload(clause.Associations).Model(&stream).Association("StreamWorkers").Find(&res)
	return res, err
}

// GetAllStreams returns all streams of the tumlive
func (d streamsDao) GetAllStreams() ([]model.Stream, error) {
	var res []model.Stream
	err := DB.Find(&res).Error
	return res, err
}

func (d streamsDao) GetCurrentLive(ctx context.Context) (currentLive []model.Stream, err error) {
	if streams, found := Cache.Get("AllCurrentlyLiveStreams"); found {
		return streams.([]model.Stream), nil
	}
	var streams []model.Stream
	if err := DB.Find(&streams, "live_now = ?", true).Error; err != nil {
		return nil, err
	}
	Cache.SetWithTTL("AllCurrentlyLiveStreams", streams, 10, time.Second)
	return streams, err
}

func (d streamsDao) GetCurrentLiveNonHidden(ctx context.Context) (currentLive []model.Stream, err error) {
	if streams, found := Cache.Get("NonHiddenCurrentlyLiveStreams"); found {
		return streams.([]model.Stream), nil
	}
	var streams []model.Stream
	if err := DB.Joins("JOIN courses ON courses.id = streams.course_id").Find(&streams,
		"live_now = ? AND visibility != ?", true, "hidden").Error; err != nil {
		return nil, err
	}
	Cache.SetWithTTL("NonHiddenCurrentlyLiveStreams", streams, 1, time.Minute)
	return streams, err
}

// GetLiveStreamsInLectureHall returns all streams that are live and in the lecture hall
func (d streamsDao) GetLiveStreamsInLectureHall(lectureHallId uint) ([]model.Stream, error) {
	var streams []model.Stream
	err := DB.Where("lecture_hall_id = ? AND live_now", lectureHallId).Find(&streams).Error
	return streams, err
}

// GetStreamsWithWatchState returns a list of streams with their progress information.
func (d streamsDao) GetStreamsWithWatchState(courseID uint, userID uint) (streams []model.Stream, err error) {
	type watchedState struct {
		Watched bool
	}
	var watchedStates []watchedState
	queriedStreams := DB.Table("streams").Where("course_id = ? and deleted_at is NULL", courseID)
	result := queriedStreams.
		Joins("left join (select watched, stream_id from stream_progresses where user_id = ?) as sp on sp.stream_id = streams.id", userID).
		Order("start desc").     // order by descending start time, this is also the order that is used in the course page.
		Session(&gorm.Session{}) // Session is required to scan multiple times

	if err = result.Scan(&streams).Error; err != nil {
		return
	}
	err = result.Scan(&watchedStates).Error
	// Updates the watch state for each stream to compensate for split query.
	for i := range streams {
		streams[i].Watched = watchedStates[i].Watched
	}
	return
}

// SetLectureHall set lecture-halls of streamIds to lectureHallID
func (d streamsDao) SetLectureHall(streamIDs []uint, lectureHallID uint) error {
	return DB.Model(&model.Stream{}).Where("id IN ?", streamIDs).Update("lecture_hall_id", lectureHallID).Error
}

// UnsetLectureHall set lecture-halls of streamIds to NULL
func (d streamsDao) UnsetLectureHall(streamIDs []uint) error {
	return DB.Model(&model.Stream{}).Where("id IN ?", streamIDs).Update("lecture_hall_id", nil).Error
}

func (d streamsDao) UpdateStream(stream model.Stream) error {
	defer Cache.Clear()
	err := DB.Model(&stream).Updates(map[string]interface{}{
		"name":         stream.Name,
		"description":  stream.Description,
		"start":        stream.Start,
		"end":          stream.End,
		"chat_enabled": stream.ChatEnabled}).Error
	return err
}

// SaveWorkerForStream associates a worker with a stream with streamID
func (d streamsDao) SaveWorkerForStream(stream model.Stream, worker model.Worker) error {
	defer Cache.Clear()
	return DB.Model(&stream).Association("StreamWorkers").Append(&worker)
}

// ClearWorkersForStream deletes all workers for a stream with streamID
func (d streamsDao) ClearWorkersForStream(stream model.Stream) error {
	defer Cache.Clear()
	return DB.Model(&stream).Association("StreamWorkers").Clear()
}

func (d streamsDao) DeleteSilences(streamID string) error {
	return DB.Delete(&model.Silence{}, "stream_id = ?", streamID).Error
}

func (d streamsDao) UpdateSilences(silences []model.Silence, streamID string) error {
	err := d.DeleteSilences(streamID)
	if err != nil {
		return err
	}
	return DB.Save(&silences).Error
}

func (d streamsDao) UpdateStreamFullAssoc(vod *model.Stream) error {
	defer Cache.Clear()
	err := DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&vod).Error
	return err
}

func (d streamsDao) SetStreamNotLiveById(streamID uint) error {
	defer Cache.Clear()
	return DB.Debug().Exec("UPDATE `streams` SET `live_now`='0' WHERE id = ?", streamID).Error
}

// SetStreamLiveNowTimestampById stores timestamp when stream is going live.
func (d streamsDao) SetStreamLiveNowTimestampById(streamID uint, liveNowTimestamp time.Time) error {
	defer Cache.Clear()
	return DB.Model(model.Stream{}).Where("id = ?", streamID).Updates(map[string]interface{}{"LiveNowTimestamp": liveNowTimestamp}).Error
}

func (d streamsDao) SavePauseState(streamID uint, paused bool) error {
	defer Cache.Clear()
	return DB.Model(model.Stream{}).Where("id = ?", streamID).Update("Paused", paused).Error
}

// SaveEndedState updates the boolean Ended field of a stream model to the value of hasEnded when a stream finishes.
func (d streamsDao) SaveEndedState(streamID uint, hasEnded bool) error {
	defer Cache.Clear()
	return DB.Model(&model.Stream{}).Where("id = ?", streamID).Updates(map[string]interface{}{"Ended": hasEnded}).Error
}

func (d streamsDao) SaveCOMBURL(stream *model.Stream, url string) {
	Cache.Clear()
	DB.Model(stream).Updates(map[string]interface{}{"playlist_url": url, "live_now": 1, "recording": 0})
	Cache.Clear()
}

func (d streamsDao) SaveCAMURL(stream *model.Stream, url string) {
	Cache.Clear()
	DB.Model(stream).Updates(map[string]interface{}{"playlist_url_cam": url, "live_now": 1, "recording": 0})
	Cache.Clear()
}

func (d streamsDao) SavePRESURL(stream *model.Stream, url string) {
	Cache.Clear()
	DB.Model(stream).Updates(map[string]interface{}{"playlist_url_pres": url, "live_now": 1, "recording": 0})
	Cache.Clear()
}

func (d streamsDao) ToggleVisibility(streamId uint, private bool) error {
	return DB.Model(&model.Stream{}).Where("id = ?", streamId).Updates(map[string]interface{}{"private": private}).Error
}

func (d streamsDao) SaveStream(vod *model.Stream) error {
	defer Cache.Clear()
	// todo: what is this?
	err := DB.Model(&vod).Updates(model.Stream{
		Name:             vod.Name,
		Description:      vod.Description,
		CourseID:         vod.CourseID,
		LiveNowTimestamp: vod.LiveNowTimestamp,
		Start:            vod.Start,
		End:              vod.End,
		RoomName:         vod.RoomName,
		RoomCode:         vod.RoomCode,
		EventTypeName:    vod.EventTypeName,
		PlaylistUrl:      vod.PlaylistUrl,
		PlaylistUrlPRES:  vod.PlaylistUrlPRES,
		PlaylistUrlCAM:   vod.PlaylistUrlCAM,
		LiveNow:          vod.LiveNow,
		Recording:        vod.Recording,
		Chats:            vod.Chats,
		Stats:            vod.Stats,
		Units:            vod.Units,
		VodViews:         vod.VodViews,
		StartOffset:      vod.StartOffset,
		EndOffset:        vod.EndOffset,
		Silences:         vod.Silences,
		Files:            vod.Files,
		Paused:           vod.Paused,
		Duration:         vod.Duration,
		ThumbInterval:    vod.ThumbInterval,
	}).Error
	return err
}

func (d streamsDao) RemoveTranscodingProgress(streamVersion model.StreamVersion, streamId uint) error {
	return DB.Unscoped().Where("version = ? AND stream_id = ?", streamVersion, streamId).Delete(&model.TranscodingProgress{}).Error
}

func (d streamsDao) DeleteStream(streamID string) {
	DB.Where("id = ?", streamID).Delete(&model.Stream{})
	Cache.Clear()
}

func (d streamsDao) DeleteUnit(id uint) {
	defer Cache.Clear()
	DB.Delete(&model.StreamUnit{}, id)
}

func (d streamsDao) DeleteStreamsWithTumID(ids []uint) {
	// transaction for performance
	_ = DB.Transaction(func(tx *gorm.DB) error {
		for i := range ids {
			tx.Where("tum_online_event_id = ?", ids[i]).Delete(&model.Stream{})
		}
		return nil
	})
}
