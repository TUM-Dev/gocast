package runner

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tum-dev/gocast/runner/config"
)

func (r *Runner) livestreamCleanup() {
	r.log.Info("Starting livestream cleanup")
	for !r.draining {
		livePath := config.Config.SegmentPath
		streamDirs, err := os.ReadDir(livePath)
		if err != nil {
			r.log.Error("Error reading livestream cleanup dir", "path", livePath)
		}
		for _, streamDir := range streamDirs {
			shouldDelete := false
			deletionError := false
			if streamDir.IsDir() {
				streamVersionDirs, err := os.ReadDir(filepath.Join(livePath, streamDir.Name()))
				if err != nil {
					r.log.Error("Error reading livestream cleanup dir", "name", streamDir.Name())
				}
				for _, streamVersionDir := range streamVersionDirs {
					if streamVersionDir.IsDir() {
						files, err := os.ReadDir(filepath.Join(livePath, streamDir.Name(), streamVersionDir.Name()))
						if err != nil {
							r.log.Error("Error reading livestream cleanup dir", "name", streamDir.Name())
							deletionError = true
							continue
						}
						for _, file := range files {
							if !file.IsDir() {
								if strings.HasPrefix(file.Name(), ".del-") {
									// Read time from filename and delete if necessary
									delTime, err := time.Parse(time.RFC3339, file.Name()[5:])
									if err != nil {
										r.log.Error("Error parsing time string", "name", file.Name())
										deletionError = true
									} else {
										if delTime.Before(time.Now()) {
											shouldDelete = true
											r.log.Info("Deleting livestream as VoD healthy", "name", file.Name())
											err = os.RemoveAll(filepath.Join(livePath, streamDir.Name(), streamVersionDir.Name()))
											if err != nil {
												r.log.Error("Error removing time string", "name", file.Name())
												deletionError = true
											}
										}
									}
									break
								} else if strings.HasPrefix(file.Name(), ".keep-") {
									// Read time from filename and move if necessary
									keepTime, err := time.Parse(time.RFC3339, file.Name()[6:])
									if err != nil {
										r.log.Error("Error parsing time string", file.Name())
										deletionError = true
									} else {
										if keepTime.Before(time.Now()) {
											shouldDelete = true
											newErrorFolder := filepath.Join(config.Config.ErrorPath, streamDir.Name())
											err := os.MkdirAll(newErrorFolder, os.ModePerm)
											if err != nil {
												r.log.Error("Error creating errors cleanup dir", "name", streamDir.Name())
												deletionError = true
											}
											err = os.Rename(filepath.Join(livePath, streamDir.Name(), streamVersionDir.Name()), filepath.Join(newErrorFolder, streamVersionDir.Name()))
											if err != nil {
												r.log.Error("Error moving error livestream", "name", streamDir.Name())
												deletionError = true
											}
										}
									}
									break
								}
							}
						}
					}
				}
			}
			if !deletionError && shouldDelete {
				r.log.Info("Deleting whole livestream dir as VoD healthy", "name", streamDir.Name())
				err = os.RemoveAll(filepath.Join(livePath, streamDir.Name()))
				if err != nil {
					r.log.Error("Error removing livestream cleanup dir", "name", streamDir.Name())
				}
			}
		}
		// TODO: Set this higher
		time.Sleep(1 * time.Minute)
	}
	r.log.Debug("Stopping livestream cleanup")
}
