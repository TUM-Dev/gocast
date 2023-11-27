// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

import (
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/model"
	"github.com/golang/protobuf/ptypes"
    "github.com/TUM-Dev/gocast/dao"
    )	

// ParseUserToProto converts a User model to its protobuf representation.
func ParseUserToProto(u *model.User) *protobuf.User {
        user := &protobuf.User{
            Id:                 uint32(u.ID),
            Name:               u.Name,
            Email:              u.Email.String,
            MatriculationNumber: u.MatriculationNumber,
            LrzID:              u.LrzID,
            Role:               uint32(u.Role),
            Settings:           []*protobuf.UserSetting{},
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
                Id:       uint32(setting.ID),
                UserID:   uint32(setting.UserID),
                Type:     protobuf.UserSettingType(setting.Type),
                Value:    setting.Value,
        }
    }
    
// ParseBookmarkToProto converts a Bookmark model to its protobuf representation.
func ParseBookmarkToProto(bookmark model.Bookmark) *protobuf.Bookmark {
    return &protobuf.Bookmark{
            Description:	bookmark.Description,
            Hours:			uint32(bookmark.Hours),
            Minutes:     	uint32(bookmark.Minutes),
            Seconds:    	uint32(bookmark.Seconds),
            UserID:    		uint32(bookmark.UserID),
            StreamID:   	uint32(bookmark.StreamID),
    }
}
    
// ParseCourseToProto converts a Course model to its protobuf representation.
func ParseCourseToProto(course model.Course) *protobuf.Course {
	return &protobuf.Course{
		Id:           uint32(course.ID),
		Name:         course.Name,
		Slug:         course.Slug,
		Semester: &protobuf.Semester{
			Year: uint32(course.Year),
			TeachingTerm: course.TeachingTerm,
		},
		TUMOnlineIdentifier: course.TUMOnlineIdentifier,
		VODEnabled: course.VODEnabled,
		DownloadsEnabled: course.DownloadsEnabled,
		ChatEnabled: course.ChatEnabled,
		AnonymousChatEnabled: course.AnonymousChatEnabled,
		ModeratedChatEnabled: course.ModeratedChatEnabled,
		VodChatEnabled: course.VodChatEnabled,
		CameraPresetPreferences: course.CameraPresetPreferences,
		SourcePreferences: course.SourcePreferences,
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
func ParseStreamToProto(stream model.Stream) (*protobuf.Stream, error) {
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
        Id:                 uint64(stream.ID),
        Name:               stream.Name,
        Description:        stream.Description,
        CourseID:           uint32(stream.CourseID),
        Start:              start,
        End:                end,
        ChatEnabled:        stream.ChatEnabled,
        RoomName:           stream.RoomName,
        RoomCode:           stream.RoomCode,
        EventTypeName:      stream.EventTypeName,
        TUMOnlineEventID:   uint32(stream.TUMOnlineEventID),
        SeriesIdentifier:   stream.SeriesIdentifier,
        PlaylistUrl:        stream.PlaylistUrl,
        PlaylistUrlPRES:    stream.PlaylistUrlPRES,
        PlaylistUrlCAM:     stream.PlaylistUrlCAM,
        LiveNow:            stream.LiveNow,
        LiveNowTimestamp:   liveNowTimestamp,
        Recording:          stream.Recording,
        Premiere:           stream.Premiere,
        Ended:              stream.Ended,
        VodViews:           uint32(stream.VodViews),
        StartOffset:        uint32(stream.StartOffset),
        EndOffset:          uint32(stream.EndOffset),
    }, nil

	if stream.Duration.Valid {
		s.Duration = int32(stream.Duration.Int32)
    }

	return s, err
}
