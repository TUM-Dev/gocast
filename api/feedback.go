package api

import (
	"encoding/json"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/bot"
	"io/ioutil"
	"net/http"
	"strconv"
)

/*func configFeedbackRouter(router *gin.Engine) {
	router.GET("/api/feedback", submitFeedback)
}*/

func submitFeedback(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	type userFeedback struct {
		Feedback string `json:"feedback"`
	}

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		sentry.CaptureException(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var feedback userFeedback
	err = json.Unmarshal(body, &feedback)

	feedbackMessage := bot.FeedbackMessage{
		Feedback:   feedback.Feedback,
		UserID:     strconv.Itoa(int(tumLiveContext.User.ID)),
		AuthorName: tumLiveContext.User.Name,
	}

	var bot bot.Bot
	err = bot.SendUserFeedback(feedbackMessage)
	if err != nil {
		sentry.CaptureException(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

}
