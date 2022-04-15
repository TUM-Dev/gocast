package bot

import (
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"strings"
)

type MessageType int

const (
	Feedback MessageType = iota + 1
	Info
)

type Bot struct {
	Method MessagingMethod
}

type Message struct {
	Text   string
	Type   MessageType
	Method MessagingMethod
}

// InfoMessage contains all information that is needed for a debugging message.
// This should later be extended with a custom message field that can be filled on the stream page.
type InfoMessage struct {
	CourseName  string
	LectureHall string
	StreamUrl   string
	CombIP      string
	CameraIP    string
}

// FeedbackMessage represents a message that users can send via the website if they want to give feedback.
type FeedbackMessage struct {
	Feedback   string
	UserID     string
	AuthorName string
}

// MessagingMethod provides a generic interface for different message providers e.g. Matrix
type MessagingMethod interface {
	SendBotMessage(message Message) error
}

// SetMessagingMethod sets the provider method for sending messages e.g. Matrix
func (b *Bot) SetMessagingMethod(method MessagingMethod) {
	b.Method = method
}

func (b *Bot) SendMessage(message Message) error {
	return b.Method.SendBotMessage(message)
}

// GenerateInfoText generates a formatted issue text.
func GenerateInfoText(botInfo InfoMessage) string {
	combIP := strings.Split(botInfo.CombIP, "/")[0] // URL has /extron[...]
	return "🚨 **Technical problem**\n\n" +
		"* **Course name**: " + botInfo.CourseName + "\n" +
		"* **Lecture hall**: " + botInfo.LectureHall + "\n" +
		"* **Stream URL**: " + botInfo.StreamUrl + "\n" +
		"* **Combined IP**: " + "[" + combIP + "](http://" + combIP + ")\n" +
		"* **Camera IP**: " + "[" + botInfo.CameraIP + "](http://" + botInfo.CameraIP + ")\n"
}

// GenerateFeedbackText generates formatted feedback text.
func GenerateFeedbackText(feedback FeedbackMessage) string {
	return "💬 **Feedback**\n\n" +
		"* **User ID**: " + feedback.UserID + "\n" +
		"* **Name:**: " + feedback.AuthorName + "\n" +
		"* **Feedback**: " + feedback.Feedback
}

// getFormattedMessageText generates a HTML styled message bot info
func getFormattedMessageText(message string) string {
	unsafe := blackfriday.Run([]byte(message))
	// Sanitization already in place since we want to edit user generated content soon
	html := bluemonday.
		UGCPolicy().
		AddTargetBlankToFullyQualifiedLinks(true).
		SanitizeBytes(unsafe)
	return string(html)
}

func (b *Bot) SendUserFeedback(feedback FeedbackMessage) error {
	message := Message{
		Text:   getFormattedMessageText(feedback.Feedback),
		Type:   Feedback,
		Method: &Matrix{},
	}
	return b.SendMessage(message)
}

func (b *Bot) SendInfoMessage(info InfoMessage) error {
	message := Message{
		Text: getFormattedMessageText(GenerateInfoText(info)),
		Type: Info,
	}
	return b.SendMessage(message)
}
