package bot

import (
	"errors"
	"strings"
)

// BotInfo contains all information that is needed for a debugging message
// TODO: This should later be extended with a custom message field that can be filled on the stream page
type BotInfo struct {
	CourseName   string
	LectureHall  string
	StreamUrl    string
	CombIP       string
	CameraIP     string
	BotMessaging BotMessagingMethod
}

// BotMessagingMethod provides a generic interface for different message providers e.g. Matrix
type BotMessagingMethod interface {
	SendBotMessage(botInfo BotInfo) error
}

// SetMessagingMethod sets the provider method for sending messages e.g. matrix
func (c *BotInfo) SetMessagingMethod(messaging BotMessagingMethod) {
	c.BotMessaging = messaging
}

// getMessageText generates an unformatted issue text.
func getMessageText(botInfo BotInfo) string {
	return "🚨 Technical problem:" + "\n" +
		"Course name: " + botInfo.CourseName + "\n" +
		"Lecture hall: " + botInfo.LectureHall + "\n" +
		"URL: " + botInfo.StreamUrl + "\n" +
		"Lecture hall: " + botInfo.LectureHall + "\n" +
		"CombinedIP: " + "http://" + botInfo.CombIP + "\n" +
		"CameraIP: " + "http://" + botInfo.CameraIP + "\n"
}

// getFormattedMessageText generates a html-formatted issue text.
// For Matrix refer to https://spec.matrix.org/v1.2/client-server-api/#mroommessage-msgtypes
// for formatting options that clients usually support.
func getFormattedMessageText(botInfo BotInfo) string {
	combIP := strings.Split(botInfo.CombIP, "/")[0] // URL has /extron[...] attached, we just want the IP of the device
	return "🚨 <strong>Technical problem</strong>:" + "<br>" +
		"Course name: " + "<em>" + botInfo.CourseName + "</em>" + "<br>" +
		"Lecture hall: " + "<em>" + botInfo.LectureHall + "</em>" + "<br>" +
		"URL: " + botInfo.StreamUrl + "<br>" +
		"CombinedIP: " + "http://" + combIP + "<br>" +
		"CameraIP: " + "http://" + botInfo.CameraIP + "<br>"
}

// BotUpdate sends a message containing data from a stream, course and lecture hall
// to a messaging server. Right now only matrix is supported.
func (c *BotInfo) BotUpdate(info BotInfo) error {
	// Currently only supported for lecture hall streams
	if info.LectureHall == "" {
		return errors.New("sending bot messages is not supported for selfstreams")
	}
	err := c.BotMessaging.SendBotMessage(info)
	return err
}
