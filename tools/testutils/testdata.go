package testutils

import (
	"errors"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/mock_tools"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"gorm.io/gorm"
	"testing"
	"time"
)

// Misc
var (
	StartTime             = time.Now()
	TUMLiveContextStudent = tools.TUMLiveContext{User: &Student}
	TUMLiveContextAdmin   = tools.TUMLiveContext{User: &Admin}
)

// Models
var (
	Student          = model.User{Model: gorm.Model{ID: 42}, Role: model.StudentType}
	Admin            = model.User{Model: gorm.Model{ID: 0}, Role: model.AdminType}
	EmptyLectureHall = model.LectureHall{}
	LectureHall      = model.LectureHall{
		Model:          gorm.Model{ID: uint(1)},
		Name:           "FMI_HS1",
		FullName:       "MI HS1",
		CombIP:         "127.0.0.1/extron3",
		PresIP:         "127.0.0.1/extron1",
		CamIP:          "127.0.0.1/extron2",
		CameraIP:       "127.0.0.1",
		RoomID:         0,
		PwrCtrlIp:      "http://pwrctrlip.in.test.de",
		LiveLightIndex: 0,
	}
	CameraPreset = model.CameraPreset{
		Name:          "Home",
		PresetID:      1,
		Image:         "ccc47fae-847c-4a91-8a65-b26cbae6fbe2.jpg",
		LectureHallId: LectureHall.ID,
		IsDefault:     false,
	}
	CourseFPV = model.Course{
		Model:                gorm.Model{ID: uint(40)},
		UserID:               1,
		Name:                 "Funktionale Programmierung und Verifikation (IN0003)",
		Slug:                 "fpv",
		Year:                 0,
		TeachingTerm:         "W",
		TUMOnlineIdentifier:  "2020",
		LiveEnabled:          true,
		VODEnabled:           false,
		DownloadsEnabled:     false,
		ChatEnabled:          false,
		AnonymousChatEnabled: false,
		ModeratedChatEnabled: false,
		VodChatEnabled:       false,
		Visibility:           "public",
	}
	StreamFPVLive = model.Stream{
		Model:            gorm.Model{ID: 1969},
		Name:             "Lecture 1",
		Description:      "First official stream",
		CourseID:         CourseFPV.ID,
		Start:            StartTime,
		End:              StartTime.Add(time.Hour),
		RoomName:         "00.08.038, Seminarraum",
		RoomCode:         "5608.EG.038",
		EventTypeName:    "Abhaltung",
		TUMOnlineEventID: 888261337,
		SeriesIdentifier: "",
		StreamKey:        "0dc3d-1337-1194-38f8-1337-7f16-bbe1-1337",
		PlaylistUrl:      "https://url",
		PlaylistUrlPRES:  "https://url",
		PlaylistUrlCAM:   "https://url",
		LiveNow:          true,
		LectureHallID:    LectureHall.ID,
		Files:            []model.File{Attachment, AttachmentInvalidPath},
		VideoSections: []model.VideoSection{
			{
				Description:  "Introduction",
				StartHours:   0,
				StartMinutes: 0,
				StartSeconds: 0,
				StreamID:     CourseFPV.ID,
			},
			{
				Description:  "Proofs",
				StartHours:   1,
				StartMinutes: 33,
				StartSeconds: 7,
				StreamID:     CourseFPV.ID,
			},
		},
	}
	StreamFPVNotLive = model.Stream{
		Model:            gorm.Model{ID: 1969},
		Name:             "Lecture 1",
		Description:      "First official stream",
		CourseID:         CourseFPV.ID,
		Start:            StartTime,
		End:              StartTime.Add(time.Hour),
		RoomName:         "00.08.038, Seminarraum",
		RoomCode:         "5608.EG.038",
		EventTypeName:    "Abhaltung",
		TUMOnlineEventID: 888261337,
		SeriesIdentifier: "",
		StreamKey:        "0dc3d-1337-1194-38f8-1337-7f16-bbe1-1337",
		PlaylistUrl:      "https://url",
		PlaylistUrlPRES:  "https://url",
		PlaylistUrlCAM:   "https://url",
		LiveNow:          false,
		LectureHallID:    LectureHall.ID,
	}
	SelfStream = model.Stream{
		Model:            gorm.Model{ID: 420},
		Name:             "Selfstream1",
		Description:      "First selfstream",
		CourseID:         CourseFPV.ID,
		Start:            StartTime,
		End:              StartTime.Add(time.Hour),
		TUMOnlineEventID: 888261337,
		SeriesIdentifier: "",
		StreamKey:        "0dc3d-1337-1194-38f8-1337-7f16-bbe1-1111",
		PlaylistUrl:      "https://url",
		PlaylistUrlPRES:  "https://url",
		PlaylistUrlCAM:   "https://url",
		LiveNow:          false,
	}
	Worker1 = model.Worker{
		WorkerID: "ed067fa3-2364-4dcd-bfd2-e0ffb8d751d4",
		Host:     "worker1.local",
		Status:   "",
		Workload: 0,
		LastSeen: time.Now(),
	}
	Worker2 = model.Worker{
		WorkerID: "ed067fa3-2364-4dcd-bfd2-e0ffb8d751d4",
		Host:     "worker2.local",
		Status:   "",
		Workload: 0,
		LastSeen: time.Now(),
	}
	AdminToken = model.Token{
		UserID: Admin.ID,
		User:   Admin,
		Token:  "ed067f11-1337-4dcd-bfd2-4201b8d751d4",
		Scope:  model.TokenScopeAdmin,
	}
	Attachment = model.File{
		StreamID: 1969,
		Path:     "/tmp/test.txt",
		Filename: "test.txt",
		Type:     model.FILETYPE_ATTACHMENT,
	}
	AttachmentInvalidPath = model.File{
		StreamID: 1969,
		Path:     "/tmp/i_do_not_exist.txt",
		Filename: "i_do_not_exist.txt",
		Type:     model.FILETYPE_ATTACHMENT,
	}
	InfoPage = model.InfoPage{
		Model:      gorm.Model{ID: 1},
		Name:       "Data Privacy",
		RawContent: "#data privacy",
		Type:       model.INFOPAGE_MARKDOWN,
	}
	FPVNotLiveVideoSeekChunk1 = model.VideoSeekChunk{
		ChunkIndex: 1,
		StreamID:   StreamFPVNotLive.ID,
		Hits:       247,
	}
	FPVNotLiveVideoSeekChunk2 = model.VideoSeekChunk{
		ChunkIndex: 2,
		StreamID:   StreamFPVNotLive.ID,
		Hits:       112,
	}
	FPVNotLiveVideoSeekChunk3 = model.VideoSeekChunk{
		ChunkIndex: 3,
		StreamID:   StreamFPVNotLive.ID,
		Hits:       788,
	}
)

func GetStreamMock(t *testing.T) dao.StreamsDao {
	streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	streamsMock.
		EXPECT().
		GetStreamByID(gomock.Any(), fmt.Sprintf("%d", StreamFPVLive.ID)).
		Return(StreamFPVLive, nil).AnyTimes()
	streamsMock.
		EXPECT().
		GetWorkersForStream(StreamFPVLive).
		Return([]model.Worker{Worker1, Worker2}, nil).AnyTimes()
	streamsMock.
		EXPECT().
		ClearWorkersForStream(StreamFPVLive).
		Return(nil).AnyTimes()
	streamsMock.
		EXPECT().
		ToggleVisibility(StreamFPVLive.ID, gomock.Any()).
		Return(nil).AnyTimes()
	streamsMock.
		EXPECT().
		GetCurrentLive(gomock.Any()).
		Return([]model.Stream{StreamFPVLive}, nil).AnyTimes()
	return streamsMock
}

func GetAuditMock(t *testing.T) dao.AuditDao {
	auditMock := mock_dao.NewMockAuditDao(gomock.NewController(t))
	return auditMock
}
func GetCoursesMock(t *testing.T) dao.CoursesDao {
	coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	coursesMock.
		EXPECT().
		GetCourseById(gomock.Any(), CourseFPV.ID).
		Return(CourseFPV, nil).
		AnyTimes()
	return coursesMock
}

func GetLectureHallMock(t *testing.T) dao.LectureHallsDao {
	lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
	lectureHallMock.
		EXPECT().
		GetLectureHallByID(LectureHall.ID).
		Return(LectureHall, nil)
	return lectureHallMock
}

func GetLectureHallMockError(t *testing.T) dao.LectureHallsDao {
	lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
	lectureHallMock.
		EXPECT().
		GetLectureHallByID(gomock.Any()).
		Return(model.LectureHall{}, errors.New(""))
	return lectureHallMock
}

func GetTokenMock(t *testing.T) dao.TokenDao {
	tokenMock := mock_dao.NewMockTokenDao(gomock.NewController(t))
	tokenMock.
		EXPECT().
		GetToken(AdminToken.Token).
		Return(AdminToken, nil)
	tokenMock.
		EXPECT().
		TokenUsed(AdminToken).
		Return(nil)
	return tokenMock
}

func GetFileMock(t *testing.T) dao.FileDao {
	fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
	fileMock.
		EXPECT().
		GetFileById(fmt.Sprintf("%d", Attachment.ID)).
		Return(Attachment, nil)
	fileMock.
		EXPECT().
		DeleteFile(Attachment.ID).
		Return(nil)
	return fileMock
}

func GetProgressMock(t *testing.T) dao.ProgressDao {
	progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))
	progressMock.
		EXPECT().
		SaveProgresses(gomock.Any()).
		Return(nil).AnyTimes()
	return progressMock
}

func GetPresetUtilityMock(ctrl *gomock.Controller) tools.PresetUtility {
	mockPresetUtility := mock_tools.NewMockPresetUtility(ctrl)
	mockPresetUtility.EXPECT().FetchLHPresets(LectureHall).Return().AnyTimes()
	mockPresetUtility.EXPECT().TakeSnapshot(CameraPreset).Return().AnyTimes()
	return mockPresetUtility
}
