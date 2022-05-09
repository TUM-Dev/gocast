package bot

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"strings"
	"time"
)

// Bot is the bot that will be used to send messages to the chat.
type Bot struct {
	Method MessageProvider
}

// Message is a generic message that will be forwarded via the implementation specified via ProviderMethod.
type Message struct {
	Text           string
	Prio           bool
	ProviderMethod MessageProvider
}

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
	User        model.User
}

var issuesPerStream = make(map[uint][]time.Time)

// MessageProvider provides a generic interface for different message providers e.g. Matrix
type MessageProvider interface {
	SendBotMessage(message Message) error
}

// SetMessagingMethod sets the provider method for sending messages e.g. Matrix
func (b *Bot) SetMessagingMethod(method MessageProvider) {
	b.Method = method
}

// SendMessage sends a message via the bot that abstracts the provider.
func (b *Bot) SendMessage(message Message) error {
	return b.Method.SendBotMessage(message)
}

// SendAlert sends an alert message to the bot e.g. via Matrix.
func (b *Bot) SendAlert(alert AlertMessage) error {
	issuesPerStream[alert.Stream.ID] = append(issuesPerStream[alert.Stream.ID], time.Now())
	message := Message{
		Text: getFormattedMessageText(GenerateInfoText(alert)),
		Prio: hasConsecutiveReports(alert.Stream.ID) || alert.IsLecturer,
	}
	return b.SendMessage(message)
}

// GenerateInfoText generates a formatted issue text, should be visible on any client that supports markdown and HTML.
func GenerateInfoText(botInfo AlertMessage) string {
	combIP := strings.Split(botInfo.CombIP, "/")[0] // URL has /extron[...]

	var infoText string

	infoText += "🚨 **Technical problem**\n\n" +
		"<table><tr><th>Categories</th><td>" + botInfo.Categories + "</td></tr>" +
		"<tr><th>Course name</th><td>" + botInfo.CourseName + "</td></tr>" +
		"<tr><th>Stream URL</th><td>" + botInfo.StreamUrl + "</td></tr>" +
		"<tr><th>Description</th><td>" + botInfo.Comment + "</td></tr>"

	if !botInfo.Stream.IsSelfStream() {
		if botInfo.LectureHall != "" {
			infoText += "<tr><th>Lecture hall</th><td>" + botInfo.LectureHall + "</td></tr>"
		}
		if combIP != "" {
			infoText += "<tr><th>Combined IP</th><td>" + combIP + "</td></tr>"
		}
		if botInfo.CameraIP != "" {
			infoText += "<tr><th>Camera IP</th><td>" + botInfo.CameraIP + "</td></tr>"
		}
	}
	infoText += "</table>📢 **Contact information**\n\n<table>"
	// Has the person that reported the issue entered custom contact data?
	if botInfo.Name != "" {
		infoText += "<tr><th>Name</th><td>" + botInfo.User.Name + "</td></tr>"
	} else if botInfo.User.Name != "" {
		if botInfo.User.LastName != nil {
			infoText += "<tr><th>Name</th><td>" + botInfo.User.Name + " " + *botInfo.User.LastName + "</td></tr>"
		} else {
			infoText += "<tr><th>Name</th><td>" + botInfo.User.Name + "</td></tr>"
		}
	}
	if botInfo.Email != "" {
		infoText += "<tr><th>Email</th><td>" + botInfo.Email + "</td></tr>"
	} else if botInfo.User.Email.Valid {
		infoText += "<th>Email</th><td>" + botInfo.User.Email.String + "</td></tr>"
	}
	if botInfo.PhoneNumber != "" {
		infoText += "<tr><th>Phone</th><td>" + botInfo.PhoneNumber + "</td></tr>"
	}
	infoText += "</table>"

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
