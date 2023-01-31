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
	CameraPresetDao
	ChatDao
	FileDao
	StreamsDao
	CoursesDao
	WorkerDao
	LectureHallsDao
	UsersDao
	UploadKeyDao
	ProgressDao
	ServerNotificationDao
	TokenDao
	NotificationsDao
	IngestServerDao
	VideoSectionDao
	VideoSeekDao
	KeywordDao
	SearchDao
	// AuditDao.Find(...) seems like a nice api, find can be used in other dao as well if type is not embedded
	AuditDao AuditDao
	InfoPageDao
	BookmarkDao BookmarkDao
	SubtitlesDao
}

func NewDaoWrapper() DaoWrapper {
	return DaoWrapper{
		CameraPresetDao:       NewCameraPresetDao(),
		ChatDao:               NewChatDao(),
		FileDao:               NewFileDao(),
		StreamsDao:            NewStreamsDao(),
		CoursesDao:            NewCoursesDao(),
		WorkerDao:             NewWorkerDao(),
		LectureHallsDao:       NewLectureHallsDao(),
		UsersDao:              NewUsersDao(),
		UploadKeyDao:          NewUploadKeyDao(),
		ProgressDao:           NewProgressDao(),
		ServerNotificationDao: NewServerNotificationDao(),
		TokenDao:              NewTokenDao(),
		NotificationsDao:      NewNotificiationsDao(),
		IngestServerDao:       NewIngestServerDao(),
		VideoSectionDao:       NewVideoSectionDao(),
		InfoPageDao:           NewInfoPageDao(),
		VideoSeekDao:          NewVideoSeekDao(),
		SearchDao:             NewSearchDao(),
		KeywordDao:            NewKeywordDao(),
		AuditDao:              NewAuditDao(),
		BookmarkDao:           NewBookmarkDao(),
		SubtitlesDao:          NewSubtitlesDao(),
	}
}
