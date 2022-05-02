package dao

type DaoWrapper struct {
	CameraPresetDao
	ChatDao
	FileDao
	StreamsDao
	CoursesDao
	WorkerDao
}

func NewDaoWrapper() DaoWrapper {
	return DaoWrapper{
		CameraPresetDao: NewCameraPresetDao(),
		ChatDao:         NewChatDao(),
		FileDao:         NewFileDao(),
		StreamsDao:      NewStreamsDao(),
		CoursesDao:      NewCoursesDao(),
		WorkerDao:       NewWorkerDao(),
	}
}
