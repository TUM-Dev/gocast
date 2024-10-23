package testutils

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/mock_tools"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/golang/mock/gomock"
	"gorm.io/gorm"
)

// Misc
var (
	StartTime              = time.Now()
	TUMLiveContextStudent  = tools.TUMLiveContext{User: &Student}
	TUMLiveContextLecturer = tools.TUMLiveContext{User: &Lecturer}
	TUMLiveContextAdmin    = tools.TUMLiveContext{User: &Admin}
	TUMLiveContextUserNil  = tools.TUMLiveContext{User: nil}
	TUMLiveContextEmpty    = tools.TUMLiveContext{}
)

// Models
var (
	Student          = model.User{Model: gorm.Model{ID: 42}, Role: model.StudentType, PinnedCourses: []model.Course{CourseFPV}}
	Lecturer         = model.User{Model: gorm.Model{ID: 31}, Role: model.LecturerType}
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
		LectureHallID: LectureHall.ID,
		IsDefault:     false,
	}
	CourseFPV = model.Course{
		Model:                gorm.Model{ID: uint(40)},
		UserID:               1,
		Name:                 "Funktionale Programmierung und Verifikation (IN0003)",
		Slug:                 "fpv",
		Year:                 2022,
		TeachingTerm:         "W",
		TUMOnlineIdentifier:  "2020",
		VODEnabled:           false,
		DownloadsEnabled:     false,
		ChatEnabled:          true,
		AnonymousChatEnabled: false,
		ModeratedChatEnabled: false,
		VodChatEnabled:       false,
		Visibility:           "public",
		Admins:               []model.User{Admin},
		Streams:              []model.Stream{StreamFPVLive, StreamFPVNotLive, SelfStream},
	}
	CourseGBS = model.Course{
		Model:                gorm.Model{ID: uint(42)},
		UserID:               1,
		Name:                 "Grundlagen: Betriebssysteme und Systemsoftware (IN0009)",
		Slug:                 "gbs",
		Year:                 0,
		TeachingTerm:         "W",
		TUMOnlineIdentifier:  "2021",
		VODEnabled:           false,
		DownloadsEnabled:     false,
		ChatEnabled:          true,
		AnonymousChatEnabled: false,
		ModeratedChatEnabled: false,
		VodChatEnabled:       false,
		Visibility:           "public",
	}
	CourseTensNet = model.Course{
		Model:                gorm.Model{ID: uint(55)},
		UserID:               1,
		Name:                 "Tensor Networks (IN2388)",
		Slug:                 "TensNet",
		Year:                 2023,
		TeachingTerm:         "S",
		TUMOnlineIdentifier:  "2023",
		VODEnabled:           false,
		DownloadsEnabled:     false,
		ChatEnabled:          true,
		AnonymousChatEnabled: false,
		ModeratedChatEnabled: false,
		VodChatEnabled:       false,
		Visibility:           "enrolled",
		Streams:              []model.Stream{StreamTensNetLive},
	}
	StreamFPVLive = model.Stream{
		Model:            gorm.Model{ID: 1969},
		Name:             "Lecture 1",
		Description:      "First official stream",
		CourseID:         40,
		Start:            StartTime,
		End:              StartTime.Add(time.Hour),
		ChatEnabled:      true,
		RoomName:         "00.08.038, Seminarraum",
		RoomCode:         "5608.EG.038",
		EventTypeName:    "Abhaltung",
		TUMOnlineEventID: 888261337,
		SeriesIdentifier: "e00a5d01-c530-41c5-8698-e40ec6d828ef",
		StreamKey:        "0dc3d-1337-1194-38f8-1337-7f16-bbe1-1337",
		PlaylistUrl:      "https://url",
		PlaylistUrlPRES:  "https://url",
		PlaylistUrlCAM:   "https://url",
		LiveNow:          true,
		LectureHallID:    LectureHall.ID,
		Files:            []model.File{Attachment, AttachmentInvalidPath},
		Units: []model.StreamUnit{
			{
				Model:           gorm.Model{ID: 1},
				UnitName:        "Unit 1",
				UnitDescription: "First unit",
				UnitStart:       0,
				UnitEnd:         1111,
				StreamID:        1969,
			},
		},
		VideoSections: []model.VideoSection{
			{
				Description:  "Introduction",
				StartHours:   0,
				StartMinutes: 0,
				StartSeconds: 0,
			},
			{
				Description:  "Proofs",
				StartHours:   1,
				StartMinutes: 33,
				StartSeconds: 7,
			},
		},
	}
	StreamFPVNotLive = model.Stream{
		Model:            gorm.Model{ID: 1969},
		Name:             "Lecture 1",
		Description:      "First official stream",
		CourseID:         40,
		Start:            StartTime,
		End:              StartTime.Add(time.Hour),
		ChatEnabled:      true,
		RoomName:         "00.08.038, Seminarraum",
		RoomCode:         "5608.EG.038",
		EventTypeName:    "Abhaltung",
		TUMOnlineEventID: 888261337,
		SeriesIdentifier: "e00a5d01-c530-41c5-8698-e40ec6d828ef",
		StreamKey:        "0dc3d-1337-1194-38f8-1337-7f16-bbe1-1337",
		PlaylistUrl:      "https://url",
		PlaylistUrlPRES:  "https://url",
		PlaylistUrlCAM:   "https://url",
		LiveNow:          false,
		LectureHallID:    LectureHall.ID,
	}
	StreamTensNetLive = model.Stream{
		Model:       gorm.Model{ID: 3333},
		Name:        "Tensor Contraction",
		Description: "C = A . B",
		CourseID:    55,
		Start:       time.Time{},
		End:         time.Time{},
		LiveNow:     true,
	}
	StreamGBSLive = model.Stream{
		Model:            gorm.Model{ID: 96},
		Name:             "Linker & Loader",
		Description:      "Tweedback: ...",
		CourseID:         CourseGBS.ID,
		Start:            StartTime,
		End:              StartTime.Add(time.Hour),
		ChatEnabled:      true,
		TUMOnlineEventID: 888333337,
		SeriesIdentifier: "",
		StreamKey:        "0dc3d-1337-7331-4201-1337-7f16-bbe1-2222",
		PlaylistUrl:      "https://url",
		PlaylistUrlPRES:  "https://url",
		PlaylistUrlCAM:   "https://url",
		LiveNow:          true,
	}
	SelfStream = model.Stream{
		Model:            gorm.Model{ID: 420},
		Name:             "Selfstream1",
		Description:      "First selfstream",
		CourseID:         40,
		Start:            StartTime,
		End:              StartTime.Add(time.Hour),
		ChatEnabled:      true,
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
		Host:     "localhost",
		Status:   "",
		Workload: 0,
		LastSeen: time.Now(),
	}
	Worker2 = model.Worker{
		WorkerID: "ed067fa3-2364-4dcd-bfd2-e0ffb8d751d4",
		Host:     "localhost",
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
	Bookmark = model.Bookmark{
		Model:       gorm.Model{ID: 1},
		Description: "Klausurrelevant",
		Hours:       1,
		Minutes:     33,
		Seconds:     7,
		UserID:      Student.ID,
		StreamID:    StreamFPVLive.ID,
	}
	PollStreamFPVLive = model.Poll{
		Model:    gorm.Model{ID: uint(3)},
		StreamID: StreamFPVLive.ID,
		Stream:   StreamFPVLive,
		Question: "1+1=?",
		Active:   true,
		PollOptions: []model.PollOption{
			{Model: gorm.Model{ID: 0}, Answer: "2", Votes: []model.User{Student}},
			{Model: gorm.Model{ID: 1}, Answer: "3", Votes: []model.User{}},
		},
	}
	SubtitlesFPVLive = model.Subtitles{
		StreamID: StreamFPVLive.ID,
		Content:  "wonderful",
		Language: "en",
	}
)

// CreateVideoSeekData returns list of generated VideoSeekChunk and expected response object
func CreateVideoSeekData(streamId uint, chunkCount int) ([]model.VideoSeekChunk, gin.H) {
	var chunks []model.VideoSeekChunk
	var responseChunks []gin.H

	for i := 0; i < chunkCount; i++ {
		chunk := model.VideoSeekChunk{
			ChunkIndex: uint(i),
			StreamID:   streamId,
			Hits:       uint(chunkCount + i*int(math.Pow(-1.0, float64(i)))),
		}

		chunks = append(chunks, chunk)
		responseChunks = append(responseChunks, gin.H{
			"index": chunk.ChunkIndex,
			"value": chunk.Hits,
		})
	}
	return chunks, gin.H{
		"values": responseChunks,
	}
}

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
	streamsMock.
		EXPECT().
		UpdateStreamFullAssoc(gomock.Any()).
		Return(nil).
		AnyTimes()
	streamsMock.
		EXPECT().
		GetUnitByID(fmt.Sprintf("%d", StreamFPVLive.Units[0].ID)).
		Return(StreamFPVLive.Units[0], nil).
		AnyTimes()
	streamsMock.
		EXPECT().
		DeleteUnit(StreamFPVLive.Units[0].ID).
		Return().
		AnyTimes()
	streamsMock.
		EXPECT().
		SaveStream(gomock.Any()).
		Return(nil).
		AnyTimes()
	streamsMock.
		EXPECT().
		UpdateStream(gomock.Any()).
		Return(nil).
		AnyTimes()
	streamsMock.
		EXPECT().
		CreateStream(gomock.Any()).
		Return(nil).
		AnyTimes()
	return streamsMock
}

func GetCoursesMock(t *testing.T) dao.CoursesDao {
	coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	coursesMock.
		EXPECT().
		GetCourseById(gomock.Any(), CourseFPV.ID).
		Return(CourseFPV, nil).
		AnyTimes()
	coursesMock.
		EXPECT().
		GetCourseBySlugYearAndTerm(gomock.Any(), CourseFPV.Slug, CourseFPV.TeachingTerm, CourseFPV.Year).
		Return(CourseFPV, nil).
		AnyTimes()
	coursesMock.
		EXPECT().
		GetCourseAdmins(CourseFPV.ID).
		Return([]model.User{Admin, Admin}, nil).
		AnyTimes()
	coursesMock.
		EXPECT().
		AddAdminToCourse(Admin.ID, CourseFPV.ID).
		Return(nil).
		AnyTimes()
	coursesMock.
		EXPECT().
		RemoveAdminFromCourse(Admin.ID, CourseFPV.ID).
		Return(nil).
		AnyTimes()
	coursesMock.
		EXPECT().
		GetCourseByToken("t0k3n").
		Return(CourseFPV, nil).
		AnyTimes()
	coursesMock.
		EXPECT().
		CreateCourse(gomock.Any(), gomock.Any(), true).
		Return(nil).
		AnyTimes()
	coursesMock.EXPECT().DeleteCourse(CourseFPV).AnyTimes()
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

func GetAuditMock(t *testing.T) dao.AuditDao {
	auditMock := mock_dao.NewMockAuditDao(gomock.NewController(t))
	auditMock.EXPECT().Create(gomock.Any()).Return(nil)
	return auditMock
}

func GetUsersMock(t *testing.T) dao.UsersDao {
	usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
	usersMock.
		EXPECT().
		GetUserByID(gomock.Any(), Admin.ID).
		Return(Admin, nil).
		AnyTimes()
	return usersMock
}

func GetUploadKeyMock(t *testing.T) dao.UploadKeyDao {
	streamsMock := mock_dao.NewMockUploadKeyDao(gomock.NewController(t))
	streamsMock.
		EXPECT().
		CreateUploadKey(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	return streamsMock
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
