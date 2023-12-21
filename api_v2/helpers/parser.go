// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

import (
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/golang/protobuf/ptypes"
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
		Type:  protobuf.UserSettingType(setting.Type),
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
func ParseCourseToProto(c model.Course) *protobuf.Course {
	lastRecordingID := c.GetLastRecording().ID
	nextLectureID := c.GetNextLecture().ID

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

func ParseBannerAlertToProto(bannerAlert model.ServerNotification) *protobuf.BannerAlert {
	return &protobuf.BannerAlert{
		Id:        uint32(bannerAlert.ID),
		StartsAt:  bannerAlert.FormatFrom(),
		ExpiresAt: bannerAlert.FormatExpires(),
		Text:      bannerAlert.Text,
		Warn:      bannerAlert.Warn,
	}
}

func ParseFeatureNotificationToProto(featureNotification model.Notification) *protobuf.FeatureNotification {
	return &protobuf.FeatureNotification{
		Id:     uint32(featureNotification.ID),
		Title:  *featureNotification.Title,
		Body:   featureNotification.Body,
		Target: uint32(featureNotification.Target),
	}
}

// ParseSemesterToProto converts a Semester model to its protobuf representation.
func ParseSemesterToProto(semester dao.Semester) *protobuf.Semester {
	return &protobuf.Semester{
		Year:         uint32(semester.Year),
		TeachingTerm: semester.TeachingTerm,
	}
}

// ParseStreamToProto converts a Stream model to its protobuf representation.
// It returns an error if the conversion of timestamps fails.
func ParseStreamToProto(stream *model.Stream) (*protobuf.Stream, error) {
	start, err := ptypes.TimestampProto(stream.Start)
	if err != nil {
		return nil, err
	}

	end, err := ptypes.TimestampProto(stream.End)
	if err != nil {
		return nil, err
	}

	liveNowTimestamp, err := ptypes.TimestampProto(stream.LiveNowTimestamp)
	if err != nil {
		return nil, err
	}

	s, err := &protobuf.Stream{
		Id:               uint64(stream.ID),
		Name:             stream.Name,
		Description:      stream.Description,
		CourseID:         uint32(stream.CourseID),
		Start:            start,
		End:              end,
		ChatEnabled:      stream.ChatEnabled,
		RoomName:         stream.RoomName,
		RoomCode:         stream.RoomCode,
		EventTypeName:    stream.EventTypeName,
		TUMOnlineEventID: uint32(stream.TUMOnlineEventID),
		SeriesIdentifier: stream.SeriesIdentifier,
		PlaylistUrl:      stream.PlaylistUrl,
		PlaylistUrlPRES:  stream.PlaylistUrlPRES,
		PlaylistUrlCAM:   stream.PlaylistUrlCAM,
		LiveNow:          stream.LiveNow,
		LiveNowTimestamp: liveNowTimestamp,
		Recording:        stream.Recording,
		Premiere:         stream.Premiere,
		Ended:            stream.Ended,
		VodViews:         uint32(stream.VodViews),
		StartOffset:      uint32(stream.StartOffset),
		EndOffset:        uint32(stream.EndOffset),
	}, nil

	if stream.Duration.Valid {
		s.Duration = int32(stream.Duration.Int32)
	}

	return s, err
}
