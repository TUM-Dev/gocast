package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run findRec.go <stream_id>")
	}

	streamID := os.Args[1]

	gormJSONLogger := slogGorm.New()

	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		tools.Cfg.Db.User,
		tools.Cfg.Db.Password,
		tools.Cfg.Db.Host,
		tools.Cfg.Db.Port,
		tools.Cfg.Db.Database),
	), &gorm.Config{
		PrepareStmt: true,
		Logger:      gormJSONLogger,
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	dao.DB = db

	var stream model.Stream
	var course model.Course

	if err := db.Preload("StreamWorkers").First(&stream, streamID).Error; err != nil {
		log.Fatal("Stream not found:", err)
	}
	if err := db.First(&course, stream.CourseID).Error; err != nil {
		log.Fatal("Course not found:", err)
	}

	year, month, day := stream.Start.Date()

	searchDate := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	slug := course.Slug
	directories := []string{
		"/var/lib/docker/volumes/live_recordings/_data",
		"/srv/cephfs/livestream/TUM-Live/needs-fix",
	}

	for _, worker := range stream.StreamWorkers {
		for _, dir := range directories {
			cmd := exec.Command("ssh", worker.Host, "ls", dir)
			output, err := cmd.Output()
			if err == nil {
				files := strings.Split(string(output), "\n")
				for _, file := range files {
					if strings.Contains(file, searchDate) && strings.Contains(file, slug) {
						fmt.Printf("File found on %s in %s: %s\n", worker.Host, dir, file)
					}
				}
			}
		}
	}
}
