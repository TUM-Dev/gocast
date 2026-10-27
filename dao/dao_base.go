package dao

import (
	"gorm.io/gorm"
)

var (
	// DB reference to database
	DB *gorm.DB

	Migrator = newMigrator()
)

type DaoWrapper struct {
	ChatDao
	FileDao
	StreamsDao
	CoursesDao
	LectureHallsDao
	UsersDao
	StatisticsDao
	ProgressDao
	ServerNotificationDao
	TokenDao
	NotificationsDao
	IngestServerDao
	VideoSectionDao
	VideoSeekDao
	// AuditDao.Find(...) seems like a nice api, find can be used in other dao as well if type is not embedded
	AuditDao AuditDao
	InfoPageDao
	BookmarkDao BookmarkDao
	SubtitlesDao
	TranscodingFailureDao
	EmailDao
	StreamReactionDao
	RunnerDao RunnerDao
	IntegrationDao
	IntegrationGrantDao
	MediaReapDao MediaReapDao
}

func NewDaoWrapper() DaoWrapper {
	return DaoWrapper{
		ChatDao:               NewChatDao(),
		FileDao:               NewFileDao(),
		StreamsDao:            NewStreamsDao(),
		CoursesDao:            NewCoursesDao(),
		LectureHallsDao:       NewLectureHallsDao(),
		UsersDao:              NewUsersDao(),
		StatisticsDao:         NewStatisticsDao(),
		ProgressDao:           NewProgressDao(),
		ServerNotificationDao: NewServerNotificationDao(),
		TokenDao:              NewTokenDao(),
		NotificationsDao:      NewNotificiationsDao(),
		IngestServerDao:       NewIngestServerDao(),
		VideoSectionDao:       NewVideoSectionDao(),
		InfoPageDao:           NewInfoPageDao(),
		VideoSeekDao:          NewVideoSeekDao(),
		AuditDao:              NewAuditDao(),
		BookmarkDao:           NewBookmarkDao(),
		SubtitlesDao:          NewSubtitlesDao(),
		TranscodingFailureDao: NewTranscodingFailureDao(),
		EmailDao:              NewEmailDao(),
		StreamReactionDao:     NewStreamReactionDao(),
		RunnerDao:             NewRunnerDao(),
		IntegrationDao:        NewIntegrationDao(),
		IntegrationGrantDao:   NewIntegrationGrantDao(),
		MediaReapDao:          NewMediaReapDao(),
	}
}
