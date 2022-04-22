package worker

import (
	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"github.com/joschahenningsen/TUM-Live/worker/worker/vmstat"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

// Setup starts all recurring jobs of the worker
func Setup() {
	log.SetLevel(cfg.LogLevel)
	stat := vmstat.New()
	S = &Status{
		workload:  0,
		StartTime: time.Now(),
		Jobs:      []string{},
		Stat:      stat,
	}

	var err error
	persisted, err = NewPersistable()
	if err != nil {
		log.WithError(err).Fatal("Failed to create persistable")
	}

	c := cron.New()
	_, _ = c.AddFunc("* * * * *", S.SendHeartbeat)
	_, _ = c.AddFunc("* * * * *", func() {
		err := S.Stat.Update()
		if err != nil {
			log.WithError(err).Warn("Failed to update vmstat")
		}
	})
	_, _ = c.AddFunc("0 * * * *", func() {
		log.Debugf("deleting %d old files", len(persisted.Deletable))
		var notDeleted []Deletable
		for i, deletable := range persisted.Deletable {
			if time.Since(deletable.Time) >= time.Hour*24 {
				err := os.Remove(deletable.File)
				if err != nil {
					log.WithError(err).Error("Failed to delete old recording")
				}
			} else {
				log.Debugf("keeping %s (age %v)", deletable.File, time.Since(deletable.Time))
				notDeleted = append(notDeleted, persisted.Deletable[i])
			}
		}
		err := persisted.SetDeletables(notDeleted)
		if err != nil {
			log.WithError(err).Error("Failed to update deletable")
		}
	})
	c.Start()
}
