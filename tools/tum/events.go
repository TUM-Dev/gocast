package tum

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"fmt"
	"github.com/antchfx/xmlquery"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
	"log"
	"strconv"
	"strings"
	"time"
)

func getEventsForCourse(courseID string) []Event {
	println(fmt.Sprintf("%v/rdm/course/events/xml?token=%v&courseID=%v", tools.Cfg.CampusBase, tools.Cfg.CampusToken, courseID))
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/rdm/course/events/xml?token=%v&courseID=%v", tools.Cfg.CampusBase, tools.Cfg.CampusToken, courseID))
	if err != nil {
		log.Printf("Couldn't query TUMOnline xml: %v\n", err)
		return []Event{}
	}
	var events []Event
	xmlEvents := xmlquery.Find(doc, "//cor:resource")
	for i := range xmlEvents {
		event := xmlEvents[i]
		// whoever came up with this way of parsing times is a psychopath
		start, timeErr1 := time.Parse("20060102T150405", xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='dtstart']").InnerText())
		end, timeErr2 := time.Parse("20060102T150405", xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='dtstart']").InnerText())
		if timeErr1 != nil || timeErr2 != nil {
			log.Printf("couldn't parse time: %v or %v\n", timeErr1, timeErr2)
			break
		}
		eventID64, err := strconv.Atoi(xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='singleEventID']").InnerText())
		if err != nil {
			log.Printf("EventID not an int %v\n", err)
			break
		}
		var eventTypeName, status, roomCode, roomName string
		if eventTypeNameDoc := xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='singleEventTypeName']"); eventTypeNameDoc != nil {
			eventTypeName = eventTypeNameDoc.InnerText()
		}
		if statusDoc := xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='status']"); statusDoc != nil {
			status = statusDoc.InnerText()
		}
		if roomCodeDoc := xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='adr/roomCode']"); roomCodeDoc != nil {
			roomCode = roomCodeDoc.InnerText()
		}
		if roomNameDoc := xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='adr/roomAdditionalInfo']"); roomNameDoc != nil {
			roomName = roomNameDoc.InnerText()
		}
		events = append(events, Event{
			Start:               start,
			End:                 end,
			SingleEventID:       uint(eventID64),
			SingleEventTypeName: eventTypeName,
			Status:              status,
			RoomCode:            roomCode,
			RoomName:            strings.Trim(roomName, "\n \t"),
		})
	}
	return events
}

func getEventsForCourses(courses []model.Course) {
	for i := range courses {
		course := courses[i]
		events := getEventsForCourse(course.TUMOnlineIdentifier)
		for j := range events {
			event := events[j]
			print(event.Status)
			println(event.Status=="gelöscht")
			stream, err := dao.GetStreamByTumOnlineID(context.Background(), event.SingleEventID)
			if err != nil { // Lecture does not exist yet
				if event.Status != "gelöscht" {
					println("adding a course")
					course.Streams = append(course.Streams, model.Stream{
						CourseID:         course.ID,
						Start:            event.Start,
						End:              event.End,
						RoomName:         event.RoomName,
						RoomCode:         event.RoomCode,
						EventTypeName:    event.SingleEventTypeName,
						TUMOnlineEventID: event.SingleEventID,
						StreamKey:        strings.ReplaceAll(uuid.NewV4().String(), "-", ""),
						PlaylistUrl:      "",
						LiveNow:          false,
					})
				}
			} else {
				if event.Status != "gelöscht" { // update
					stream.RoomCode = event.RoomCode
					stream.RoomName = event.RoomName
					stream.Start = event.Start
					stream.End = event.End
					stream.EventTypeName = event.SingleEventTypeName
				} else {
					stream.DeletedAt = gorm.DeletedAt{Time: time.Now()}
				}
			}
			dao.UpdateCourse(context.Background(), course)
		}
	}
}

type Event struct {
	Start               time.Time
	End                 time.Time
	SingleEventID       uint
	SingleEventTypeName string
	Status              string
	RoomCode            string
	RoomName            string
}
