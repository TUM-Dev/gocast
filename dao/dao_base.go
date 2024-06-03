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
	SchoolsDao
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
		SchoolsDao:            NewSchoolsDao(),
	}
}
