package dao

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/model"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

//go:generate mockgen -source=streams.go -destination ../mock_dao/streams.go

type StreamsDao interface {
	CreateStream(stream *model.Stream) error
	AddVodView(id string) error

	GetDueStreamsForWorkers() map[uint][]model.Stream
	GetDuePremieresForWorkers(uint) []model.Stream
	GetStreamByKey(ctx context.Context, key string) (stream model.Stream, err error)
	GetUnitByID(id string) (model.StreamUnit, error)
	GetStreamByTumOnlineID(ctx context.Context, id uint) (stream model.Stream, err error)
	GetStreamsByIds(ids []uint) ([]model.Stream, error)
	GetStreamByID(ctx context.Context, id string) (stream model.Stream, err error)
	GetWorkersForStream(stream model.Stream) ([]model.Worker, error)
	GetAllStreams() ([]model.Stream, error)
	ExecAllStreamsWithCoursesAndSubtitles(f func([]StreamWithCourseAndSubtitles))
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

	SetStreamRequested(stream model.Stream) error

	GetSoonStartingStreamInfo(user *model.User, slug string, year int, term string) (uint, string, string, error)
}

type streamsDao struct {
	db *gorm.DB
}

func (d streamsDao) SetStreamRequested(stream model.Stream) error {
	return DB.Model(&stream).Updates(map[string]interface{}{"requested": true}).Error
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

// GetDueStreamsForWorkers retrieves all streams that due to be streamed in a lecture hall, grouped by organizationID.
func (d streamsDao) GetDueStreamsForWorkers() map[uint][]model.Stream {
	var streams []struct {
		model.Stream
		OrganizationID uint
	}
	DB.Model(&model.Stream{}).
		Joins("JOIN courses c ON c.id = streams.course_id").
		Joins("JOIN organizations s ON s.id = c.organization_id").
		Where("lecture_hall_id IS NOT NULL AND start BETWEEN NOW() AND DATE_ADD(NOW(), INTERVAL 10 MINUTE)" +
			"AND live_now = false AND recording = false AND (ended = false OR ended IS NULL) AND c.deleted_at IS null").
		Scan(&streams)

	res := make(map[uint][]model.Stream)
	for _, stream := range streams {
		res[stream.OrganizationID] = append(res[stream.OrganizationID], stream.Stream)
	}
	return res
}

func (d streamsDao) GetDuePremieresForWorkers(organizationID uint) []model.Stream {
	var res []model.Stream
	DB.Joins("JOIN courses ON courses.id = streams.course_id").
		Preload("Files").
		Find(&res, "courses.organization_id = ? AND premiere AND start BETWEEN DATE_SUB(NOW(), INTERVAL 10 MINUTE) AND DATE_ADD(NOW(), INTERVAL 5 SECOND) AND live_now = false AND recording = false", organizationID)
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

type StreamWithCourseAndSubtitles struct {
	Name, Description, TeachingTerm, CourseName, Subtitles string
	ID, CourseID                                           uint
	Year                                                   int
}

// ExecAllStreamsWithCoursesAndSubtitles executes f on all streams with their courses and subtitles preloaded.
func (d streamsDao) ExecAllStreamsWithCoursesAndSubtitles(f func([]StreamWithCourseAndSubtitles)) {
	var res []StreamWithCourseAndSubtitles
	batchNum := 0
	batchSize := 100
	var numStreams int64
	DB.Where("recording").Model(&model.Stream{}).Count(&numStreams)
	for batchSize*batchNum < int(numStreams) {
		err := DB.Raw(`WITH sws AS (
				SELECT streams.id,
                    streams.name,
                    streams.description,
                    c.id as course_id,
                    c.name as course_name,
                    c.teaching_term,
                    c.year,
                    s.content as subtitles,
                    IFNULL(s.stream_id, streams.id) as sid
             	FROM streams
                      JOIN courses c ON c.id = streams.course_id
                      LEFT JOIN subtitles s ON streams.id = s.stream_id
             	WHERE streams.recording AND streams.deleted_at IS NULL
				LIMIT ? OFFSET ?
             	)
			SELECT *, GROUP_CONCAT(subtitles, '\n') AS subtitles FROM sws GROUP BY sid;`, batchSize, batchNum*batchSize).Scan(&res).Error
		if err != nil {
			fmt.Println(err)
		}
		f(res)
		batchNum++
	}
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
	queriedStreams := DB.Table("streams").Where("course_id = ? and private = false and deleted_at is NULL", courseID)
	result := queriedStreams.
		Joins("left join (select watched, stream_id from stream_progresses where user_id = ?) as sp on sp.stream_id = streams.id", userID).
		Order("start asc").      // order by ascending start time, this is also the order that is used in the course page.
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
		"chat_enabled": stream.ChatEnabled,
	}).Error
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
	return DB.Model(model.Stream{}).Where("id = ?", streamID).Updates(map[string]interface{}{"live_now": 0}).Error
	// return DB.Debug().Exec("UPDATE `streams` SET `live_now`='0' WHERE id = ?", streamID).Error
}

// SetStreamLiveNowTimestampById stores timestamp when stream is going live.
func (d streamsDao) SetStreamLiveNowTimestampById(streamID uint, liveNowTimestamp time.Time) error {
	defer Cache.Clear()
	return DB.Model(model.Stream{}).Where("id = ?", streamID).Updates(map[string]interface{}{"LiveNowTimestamp": liveNowTimestamp}).Error
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
		Duration:         vod.Duration,
		ThumbInterval:    vod.ThumbInterval,
		Private:          vod.Private,
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

func (d streamsDao) GetSoonStartingStreamInfo(user *model.User, slug string, year int, term string) (uint, string, string, error) {
	var result struct {
		CourseID  uint
		StreamKey string
		ID        string
		Slug      string
	}
	now := time.Now()
	query := DB.Table("streams").
		Select("streams.course_id, streams.stream_key, streams.id, courses.slug").
		Joins("JOIN course_admins ON course_admins.course_id = streams.course_id").
		Joins("JOIN courses ON courses.id = course_admins.course_id").
		Where("courses.slug != 'TESTCOURSE' AND streams.deleted_at IS NULL AND courses.deleted_at IS NULL AND course_admins.user_id = ? AND (streams.start <= ? AND streams.end >= ?)", user.ID, now.Add(15*time.Minute), now). // Streams starting in the next 15 minutes or currently running
		Or("courses.slug != 'TESTCOURSE' AND streams.deleted_at IS NULL AND courses.deleted_at IS NULL AND course_admins.user_id = ? AND (streams.end >= ? AND streams.end <= ?)", user.ID, now.Add(-15*time.Minute), now).     // Streams that just finished in the last 15 minutes
		Order("streams.start ASC")

	if slug != "" {
		query = query.Where("courses.slug = ?", slug)
	}

	if year != 0 {
		query = query.Where("courses.year = ?", year)
	}
	if term != "" {
		query = query.Where("courses.teaching_term = ?", term)
	}

	err := query.Limit(1).Scan(&result).Error

	if err == gorm.ErrRecordNotFound || result.StreamKey == "" || result.ID == "" || result.Slug == "" {
		stream, course, err := d.CreateOrGetTestStreamAndCourse(user)
		if err != nil {
			return 0, "", "", err
		}
		return stream.CourseID, stream.StreamKey, fmt.Sprintf("%s-%d", course.Slug, stream.ID), nil
	}

	if err != nil {
		logger.Error("Error getting soon starting stream: %v", slog.String("err", err.Error()))
		return 0, "", "", err
	}

	return result.CourseID, result.StreamKey, fmt.Sprintf("%s-%s", result.Slug, result.ID), nil
}

func (d streamsDao) CreateOrGetTestStreamAndCourse(user *model.User) (model.Stream, model.Course, error) {
	course, err := d.CreateOrGetTestCourse(user)
	if err != nil {
		return model.Stream{}, model.Course{}, err
	}

	var stream model.Stream
	err = DB.FirstOrCreate(&stream, model.Stream{
		CourseID:      course.ID,
		Name:          "Test Stream",
		Description:   "This is a test stream",
		LectureHallID: 0,
	}).Error
	if err != nil {
		return model.Stream{}, model.Course{}, err
	}

	stream.Start = time.Now().Add(5 * time.Minute)
	stream.End = time.Now().Add(1 * time.Hour)
	stream.LiveNow = true
	stream.Recording = true
	stream.LiveNowTimestamp = time.Now().Add(5 * time.Minute)
	stream.Private = true
	streamKey := uuid.NewV4().String()
	stream.StreamKey = strings.ReplaceAll(streamKey, "-", "")
	stream.LectureHallID = 1
	err = DB.Save(&stream).Error
	if err != nil {
		return model.Stream{}, model.Course{}, err
	}

	return stream, course, err
}

func (d streamsDao) CreateOrGetTestCourse(user *model.User) (model.Course, error) {
	var course model.Course
	err := DB.FirstOrCreate(&course, model.Course{
		Name:           "(" + strconv.Itoa(int(user.ID)) + ") " + user.Name + "'s Test Course",
		TeachingTerm:   "Test",
		Slug:           "TESTCOURSE",
		Year:           1234,
		OrganizationID: 1,
		Visibility:     "hidden",
		VODEnabled:     false, // TODO: Change to VODEnabled: true for default testcourse if necessary
	}).Error
	if err != nil {
		return model.Course{}, err
	}

	CoursesDao.AddOrUpdateAdminToCourse(NewDaoWrapper().CoursesDao, user.ID, course.ID)

	return course, nil
}
