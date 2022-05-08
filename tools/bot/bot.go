package bot

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"strings"
	"time"
)

type Bot struct {
	Method MessagingMethod
}

type Message struct {
	Text   string
	Prio   bool
	Method MessagingMethod
}

var issuesPerStream = make(map[uint][]time.Time)

// AlertMessage contains all information that is needed for a debugging message.
// This should later be extended with a custom message field that can be filled on the stream page.
type AlertMessage struct {
	// User defined infos (need sanitization)
	PhoneNumber string
	Email       string
	Categories  string
	Comment     string
	Name        string

	// Generated infos
	CourseName  string
	LectureHall string
	StreamUrl   string
	CombIP      string
	CameraIP    string
	IsLecturer  bool
	Stream      model.Stream
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

// SendMessage sends a message to the bot.
func (b *Bot) SendMessage(message Message) error {
	return b.Method.SendBotMessage(message)
}

// GenerateInfoText generates a formatted issue text.
func GenerateInfoText(botInfo AlertMessage) string {
	combIP := strings.Split(botInfo.CombIP, "/")[0] // URL has /extron[...]

	var infoText string
	infoText += "🚨 **Technical problem**\n\n" +
		"* **Categories:** " + botInfo.Categories + "\n" +
		"* **Course name**: " + botInfo.CourseName + "\n" +
		"* **Stream URL**: " + botInfo.StreamUrl + "\n" +
		"* **Comment**: " + botInfo.Comment + "\n\n"

	if !botInfo.Stream.IsSelfStream() {
		infoText += "* **Lecture hall**: " + botInfo.LectureHall + "\n"
		if combIP != "" {
			infoText += "* **Combined IP**: " + "[" + combIP + "](http://" + combIP + ")\n"
		}

		if botInfo.CameraIP != "" {
			infoText += "* **Camera IP**: " + "[" + botInfo.CameraIP + "](http://" + botInfo.CameraIP + ")\n"
		}
	}

	infoText +=
		"💬 **Description**\n\n" +
			botInfo.Comment + "\n\n" +
			"📢 **Contact data**\n\n" +
			"* **Name**: " + botInfo.Name + "\n" +
			"* **Phone number**: " + botInfo.PhoneNumber + "\n" +
			"* **Email**: " + botInfo.Email + "\n"

	return infoText
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

// hasConsecutiveReports checks if the stream has two reported alerts within the last 10 minutes.
func hasConsecutiveReports(streamID uint) bool {
	if len(issuesPerStream[streamID]) < 2 {
		return false
	}

	for i := range issuesPerStream[streamID] {
		if issuesPerStream[streamID][i].Sub(issuesPerStream[streamID][i+1]) < 10*time.Minute {
			return true
		}
	}
	return false
}

func (b *Bot) SendAlert(alert AlertMessage) error {
	issuesPerStream[alert.Stream.ID] = append(issuesPerStream[alert.Stream.ID], time.Now())
	message := Message{
		Text: getFormattedMessageText(GenerateInfoText(alert)),
		Prio: hasConsecutiveReports(alert.Stream.ID),
	}
	return b.SendMessage(message)
}
