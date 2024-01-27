// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

import (
	"fmt"
	"time"

	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/dao"
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
	title := ""
	if featureNotification.Title != nil {
		title = *featureNotification.Title
	}

	return &protobuf.FeatureNotification{
		Id:     uint32(featureNotification.ID),
		Title:  title,
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
func ParseStreamToProto(stream *model.Stream, downloads []model.DownloadableVod) (*protobuf.Stream, error) {
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
	}

	if stream.Duration.Valid {
		s.Duration = uint32(stream.Duration.Int32)
	}

	for _, download := range downloads {
		s.Downloads = append(s.Downloads, ParseDownloadToProto(download))
	}

	return s, nil
}

func ParseDownloadToProto(download model.DownloadableVod) *protobuf.Download {
	return &protobuf.Download{
		FriendlyName: download.FriendlyName,
		DownloadURL:  download.DownloadURL,
	}
}

// Parse Progress To Proto
func ParseProgressToProto(progress *model.StreamProgress) *protobuf.Progress {
	return &protobuf.Progress{
		Progress: float32(progress.Progress),
		Watched:  progress.Watched,
		StreamID: uint32(progress.StreamID),
		UserID:   uint32(progress.UserID),
	}
}

func ParseReactionToProto(reaction model.ChatReaction) *protobuf.ChatReaction {
	return &protobuf.ChatReaction{
		ChatID:   uint32(reaction.ChatID),
		UserID:   uint32(reaction.UserID),
		Username: reaction.Username,
		Emoji:    reaction.Emoji,
	}
}

func ParseAddressedUserToProto(addressedUser model.User) *protobuf.AddressedUser {
	return &protobuf.AddressedUser{
		Id:       uint32(addressedUser.ID),
		Username: addressedUser.Name,
	}
}

func ParseChatMessageToProto(chat model.Chat) *protobuf.ChatMessage {
	var reactions []*protobuf.ChatReaction

	for _, reaction := range chat.Reactions {
		reactions = append(reactions, ParseReactionToProto(reaction))
	}

	var replies []*protobuf.ChatMessage
	for _, reply := range chat.Replies {
		replies = append(replies, ParseChatMessageToProto(reply))
	}

	var addressedUsers []*protobuf.AddressedUser
	for _, addressedUser := range chat.AddressedToUsers {
		addressedUsers = append(addressedUsers, ParseAddressedUserToProto(addressedUser))
	}

	timestamp := timestamppb.New(chat.CreatedAt)

	return &protobuf.ChatMessage{
		Id:               uint32(chat.ID),
		StreamID:         uint32(chat.StreamID),
		UserID:           chat.UserID,
		Username:         chat.UserName,
		Message:          chat.Message,
		SanitizedMessage: chat.SanitizedMessage,
		Color:            chat.Color,
		IsVisible:        chat.IsVisible,
		Reactions:        reactions,
		Replies:          replies,
		AddressedUsers:   addressedUsers,
		IsResolved:       chat.Resolved,
		IsAdmin:          chat.Admin,
		CreatedAt:        timestamp,
	}
}

func ParseChatReactionToProto(chatReaction model.ChatReaction) *protobuf.ChatReaction {
	return &protobuf.ChatReaction{
		ChatID:   uint32(chatReaction.ChatID),
		UserID:   uint32(chatReaction.UserID),
		Username: chatReaction.Username,
		Emoji:    chatReaction.Emoji,
	}
}

func ParsePollToProto(poll model.Poll, uID uint) *protobuf.Poll {
	var pollOptions []*protobuf.PollOption

	for _, option := range poll.PollOptions {
		voted := false
		for _, user := range option.Votes {
			if user.ID == uID {
				voted = true
				break
			}
		}
		pollOptions = append(pollOptions, ParsePollOptionToProto(option, voted))
	}

	return &protobuf.Poll{
		Id:          uint32(poll.ID),
		StreamID:    uint32(poll.StreamID),
		Question:    poll.Question,
		Active:      poll.Active,
		PollOptions: pollOptions,
	}
}

func ParsePollOptionToProto(pollOption model.PollOption, voted bool) *protobuf.PollOption {
	fmt.Printf("Debug: %+v\n", pollOption)

	return &protobuf.PollOption{
		Id:     uint32(pollOption.ID),
		Answer: pollOption.Answer,
		Votes:  uint32(len(pollOption.Votes)),
		Voted:  voted,
	}
}
