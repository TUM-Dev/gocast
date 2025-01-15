// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

import (
	"time"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ParseUserToProto converts a User model to its protobuf representation.
func ParseUserToProto(u *model.User) *protobuf.User {
	user := &protobuf.User{
		Id:                  uint32(u.ID),
		Name:                u.Name,
		Email:               u.Email.String,
		MatriculationNumber: u.MatriculationNumber,
		LrzID:               u.LrzID,
		Role:                uint32(u.Role),
		Settings:            []*protobuf.UserSetting{},
	}

	if u.LastName != nil {
		user.LastName = *u.LastName
	}

	for _, setting := range u.Settings {
		user.Settings = append(user.Settings, ParseUserSettingToProto(setting))
	}

	return user
}

// ParseUserSettingToProto converts a UserSetting model to its protobuf representation.
func ParseUserSettingToProto(setting model.UserSetting) *protobuf.UserSetting {
	return &protobuf.UserSetting{
		Type:  protobuf.UserSettingType(setting.Type - 1),
		Value: setting.Value,
	}
}

// ParseBookmarkToProto converts a Bookmark model to its protobuf representation.
func ParseBookmarkToProto(b model.Bookmark) *protobuf.Bookmark {
	return &protobuf.Bookmark{
		Id:          uint32(b.ID),
		Description: b.Description,
		Hours:       uint32(b.Hours),
		Minutes:     uint32(b.Minutes),
		Seconds:     uint32(b.Seconds),
		UserID:      uint32(b.UserID),
		StreamID:    uint32(b.StreamID),
	}
}

// ParseCourseToProto converts a Course model to its protobuf representation.
func ParseCourseToProto(c model.Course, u *model.User) *protobuf.Course {
	lastRecordingID := c.GetLastRecording(u).ID
	nextLectureID := c.GetNextLecture(u).ID

	return &protobuf.Course{
		Id:   uint32(c.ID),
		Name: c.Name,
		Slug: c.Slug,
		Semester: &protobuf.Semester{
			Year:         uint32(c.Year),
			TeachingTerm: c.TeachingTerm,
		},
		TUMOnlineIdentifier:     c.TUMOnlineIdentifier,
		VODEnabled:              c.VODEnabled,
		DownloadsEnabled:        c.DownloadsEnabled,
		ChatEnabled:             c.ChatEnabled,
		AnonymousChatEnabled:    c.AnonymousChatEnabled,
		ModeratedChatEnabled:    c.ModeratedChatEnabled,
		VodChatEnabled:          c.VodChatEnabled,
		CameraPresetPreferences: c.CameraPresetPreferences,
		SourcePreferences:       c.SourcePreferences,
		LastRecordingID:         uint32(lastRecordingID),
		NextLectureID:           uint32(nextLectureID),
	}
}

// ParseSemesterToProto converts a Semester model to its protobuf representation.
func ParseSemesterToProto(semester model.Semester) *protobuf.Semester {
	return &protobuf.Semester{
		Year:         uint32(semester.Year),
		TeachingTerm: semester.TeachingTerm,
	}
}

// ParseStreamToProto converts a Stream model to its protobuf representation.
// It returns an error if the conversion of timestamps fails.
func ParseStreamToProto(stream model.Stream, downloads []model.DownloadableVod) *protobuf.Stream {
	liveNow := stream.LiveNowTimestamp.After(time.Now())

	s := &protobuf.Stream{
		Id:               uint32(stream.ID),
		Name:             stream.Name,
		Description:      stream.Description,
		CourseID:         uint32(stream.CourseID),
		Start:            timestamppb.New(stream.Start),
		End:              timestamppb.New(stream.End),
		ChatEnabled:      stream.ChatEnabled,
		RoomName:         stream.RoomName,
		RoomCode:         stream.RoomCode,
		EventTypeName:    stream.EventTypeName,
		TUMOnlineEventID: uint32(stream.TUMOnlineEventID),
		SeriesIdentifier: stream.SeriesIdentifier,
		PlaylistUrl:      stream.PlaylistUrl,
		PlaylistUrlPRES:  stream.PlaylistUrlPRES,
		PlaylistUrlCAM:   stream.PlaylistUrlCAM,
		LiveNow:          liveNow,
		LiveNowTimestamp: timestamppb.New(stream.LiveNowTimestamp),
		Recording:        stream.Recording,
		Premiere:         stream.Premiere,
		Ended:            stream.Ended,
		VodViews:         uint32(stream.VodViews),
		StartOffset:      uint32(stream.StartOffset),
		EndOffset:        uint32(stream.EndOffset),
		IsPlanned:        stream.IsPlanned(),
		IsComingUp:       stream.IsComingUp(),
		HLSUrl:           stream.HLSUrl(),
	}

	if stream.Duration.Valid {
		s.Duration = uint32(stream.Duration.Int32)
	}

	for _, download := range downloads {
		s.Downloads = append(s.Downloads, ParseDownloadToProto(download))
	}

	return s
}

func ParseLectureHallToProto(lh *model.LectureHall) *protobuf.LectureHall {
	return &protobuf.LectureHall{
		Id:   uint32(lh.ID),
		Name: lh.Name,
	}
}

func ParseDownloadToProto(download model.DownloadableVod) *protobuf.Download {
	return &protobuf.Download{
		FriendlyName: download.FriendlyName,
		DownloadURL:  download.DownloadURL,
	}
}
