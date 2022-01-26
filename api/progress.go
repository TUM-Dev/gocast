package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

var progressBuff *progressBuffer

// progressWorker contains progresses to be written to the database
type progressBuffer struct {
	Lock       sync.Mutex
	Progresses []model.StreamProgress
	interval   time.Duration
}

func newProgressBuffer() *progressBuffer {
	return &progressBuffer{
		Lock:       sync.Mutex{},
		Progresses: []model.StreamProgress{},
		interval:   time.Second * 5,
	}
}

// add new progress to the list to be flushed eventually
func (b *progressBuffer) add(progress model.StreamProgress) {
	b.Lock.Lock()
	defer b.Lock.Unlock()
	b.Progresses = append(b.Progresses, progress)
}

// flush writes the collected progresses to the database
func (b *progressBuffer) flush() error {
	b.Lock.Lock()
	defer b.Lock.Unlock()
	if len(b.Progresses) == 0 {
		return nil
	}
	err := dao.SaveProgresses(b.Progresses)
	b.Progresses = []model.StreamProgress{}
	return err
}

func (b *progressBuffer) Run() {
	for {
		time.Sleep(b.interval)
		err := b.flush()
		if err != nil {
			log.WithError(err).Error("Error flushing progress buffer")
		}
	}
}

func configProgressRouter(router *gin.Engine) {
	progressBuff = newProgressBuffer()
	go progressBuff.Run()
	router.POST("/api/progressReport", saveProgress)
}

// ProgressRequest corresponds the request that is sent by the video player when it reports its progress for VODs
type ProgressRequest struct {
	StreamID uint    `json:"streamID"`
	Progress float64 `json:"progress"` // A fraction that represents currentTime / totalTime for a given video
	// Note: To be able to save the progress, we also need the userID, but it`s already contained in the Gin context
}

func saveProgress(c *gin.Context) {
	var request ProgressRequest

	err := c.BindJSON(&request)

	if err != nil {
		log.WithError(err).Warn("Could not bind JSON from progressReport.")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	foundContext, exists := c.Get("TUMLiveContext")

	if !exists {
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	if tumLiveContext.User == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	progressBuff.add(model.StreamProgress{
		Progress: request.Progress,
		StreamID: request.StreamID,
		UserID:   tumLiveContext.User.ID,
	})
}
