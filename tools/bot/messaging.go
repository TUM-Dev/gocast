package bot

import (
	"errors"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"strings"
)

// InfoMessage contains all information that is needed for a debugging message
type InfoMessage struct {
	CourseName  string
	LectureHall string
	StreamUrl   string
	CombIP      string
	CameraIP    string
	// TODO: This should later be extended with a custom message field that can be filled on the stream page
	Method MessagingMethod
}

// MessagingMethod provides a generic interface for different message providers e.g. Matrix
type MessagingMethod interface {
	SendBotMessage(botInfo InfoMessage) error
}

// SetMessagingMethod sets the provider method for sending messages e.g. Matrix
func (c *InfoMessage) SetMessagingMethod(messaging MessagingMethod) {
	c.Method = messaging
}

// BotUpdate sends a message containing data from a stream, course and lecture hall
// to a messaging server. Right now only matrix is supported.
func (c *InfoMessage) BotUpdate(info InfoMessage) error {
	// Currently, this bot is only supported for lecture hall streams
	if info.LectureHall == "" {
		return errors.New("sending bot messages is not supported for selfstreams")
	}
	err := c.Method.SendBotMessage(info)
	return err
}

// generateInfoText generates an unformatted issue text
func generateInfoText(botInfo InfoMessage) string {
	combIP := strings.Split(botInfo.CombIP, "/")[0] // URL has /extron[...]
	return "🚨 **Technical problem**\n\n" +
		"* **Course name**: " + botInfo.CourseName + "\n" +
		"* **Lecture hall**: " + botInfo.LectureHall + "\n" +
		"* **Stream URL**: " + botInfo.StreamUrl + "\n" +
		"* **Combined IP**: " + "[" + combIP + "](http://" + combIP + ")\n" +
		"* **Camera IP**: " + "[" + botInfo.CameraIP + "](http://" + botInfo.CameraIP + ")\n"
}

// getFormattedMessageText generates a HTML styled message bot info
func getFormattedMessageText(botInfo InfoMessage) string {
	unsafe := blackfriday.Run([]byte(generateInfoText(botInfo)))
	// Sanitization already in place since we want to edit user generated content soon
	html := bluemonday.
		UGCPolicy().
		AddTargetBlankToFullyQualifiedLinks(true).
		SanitizeBytes(unsafe)
	return string(html)
}
