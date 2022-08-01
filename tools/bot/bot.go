package bot

import (
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/microcosm-cc/bluemonday"
	log "github.com/sirupsen/logrus"
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

type issueInfo struct {
	Time   time.Time
	UserID uint
}

var issuesPerStream = make(map[uint][]issueInfo)

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
func (b *Bot) SendAlert(alert AlertMessage, statsDao dao.StatisticsDao) error {
	issuesPerStream[alert.Stream.ID] = append(issuesPerStream[alert.Stream.ID], issueInfo{Time: time.Now(), UserID: alert.User.ID})
	message := Message{
		Text: getFormattedMessageText(GenerateInfoText(alert)),
		Prio: hasPrio(alert.Stream.ID, statsDao) || alert.IsLecturer,
	}
	return b.SendMessage(message)
}

// GenerateInfoText generates a formatted issue text, should be visible on any client that supports markdown and HTML.
func GenerateInfoText(botInfo AlertMessage) string {
	combIP := strings.Split(botInfo.CombIP, "/")[0] // URL has /extron[...]

	var infoText string

	infoText += "🚨 <b>Technical problem</b>\n\n" +
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
	infoText += "</table>📢 <b>Contact information</b>\n\n<table>"
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
	html := bluemonday.
		UGCPolicy().
		AddTargetBlankToFullyQualifiedLinks(true).
		SanitizeBytes([]byte(message))
	return string(html)
}

// hasPrio returns true if 1% of the current viewers of a stream with streamID reported an issue.
// When there threshold for sending an alert is greater than 1, it is also checked whether these reports are consecutive.
func hasPrio(streamID uint, statsDao dao.StatisticsDao) bool {
	distinctReports := len(issuesPerStream[streamID])

	for _, r1 := range issuesPerStream[streamID] {
		for _, r2 := range issuesPerStream[streamID] {
			if r1.UserID == r2.UserID {
				distinctReports--
			}
		}
	}

	liveViewers, err := statsDao.GetStreamNumLiveViews(context.Background(), streamID)
	if err != nil {
		sentry.CaptureException(err)
		log.WithError(err).Error("Failed to get current live viewers")
		return false
	}

	percentOfViewersWithIssue := 100 * (float64(distinctReports) / float64(liveViewers))
	// If there is more than one report, check if they are consecutive.
	if distinctReports >= 2 && len(issuesPerStream[streamID]) > 1 && percentOfViewersWithIssue >= 1 {
		consecutive := false
		// Check whether there is a duplicate User ID in issuesPerStream
		for i := range issuesPerStream[streamID] {
			// Do we have reports within in 10 minutes?
			if issuesPerStream[streamID][i].Time.Sub(issuesPerStream[streamID][i+1].Time) < 10*time.Minute {
				consecutive = true
				break
			}
		}
		return consecutive
	}
	// Returns whether at least one percent of the viewers have reported an issue within the last 10 minutes
	return percentOfViewersWithIssue >= 1
}
