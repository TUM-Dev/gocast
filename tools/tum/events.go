package tum

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"errors"
	"fmt"
	"github.com/antchfx/xmlquery"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

func getEventsForCourse(courseID string, token string) (events map[time.Time]Event, deleted []Event, err error) {
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/rdm/course/events/xml?token=%v&courseID=%v", tools.Cfg.Campus.Base, token, courseID))
	if err != nil {
		return map[time.Time]Event{}, []Event{}, err
	}
	if len(xmlquery.Find(doc, "Error")) != 0 {
		return map[time.Time]Event{}, []Event{}, errors.New("error found in xml")
	}
	eventsMap := make(map[time.Time]Event)
	var deletedEvents []Event
	xmlEvents := xmlquery.Find(doc, "//cor:resource")
	for i := range xmlEvents {
		event := xmlEvents[i]
		// whoever came up with this way of parsing times is a psychopath
		start, timeErr1 := time.ParseInLocation("20060102T150405", xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='dtstart']").InnerText(), tools.Loc)
		end, timeErr2 := time.ParseInLocation("20060102T150405", xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='dtend']").InnerText(), tools.Loc)
		if timeErr1 != nil || timeErr2 != nil {
			log.WithFields(log.Fields{"timeErr1": timeErr1, "timeErr2": timeErr2}).Warn("getEventsForCourse: couldn't parse time")
			break
		}
		eventID64, err := strconv.Atoi(xmlquery.FindOne(event, "//cor:attribute[@cor:attrID='singleEventID']").InnerText())
		if err != nil {
			log.WithField("TUMOnlineCourseID", courseID).WithError(err).Error("getEventsForCourse: EventID not an int")
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
		e := Event{
			Start:               start,
			End:                 end,
			SingleEventID:       uint(eventID64),
			SingleEventTypeName: eventTypeName,
			Status:              status,
			RoomCode:            roomCode,
			RoomName:            strings.Trim(roomName, "\n \t"),
		}
		if e.Status != "gelöscht" && e.Status != "verschoben" {
			eventsMap[start] = e
		} else {
			deletedEvents = append(deletedEvents, e)
		}
	}
	return eventsMap, deletedEvents, nil
}

func GetEventsForCourses(courses []model.Course) {
	for i := range courses {
		course := courses[i]
		var events map[time.Time]Event
		var deleted []Event
		var err error
		for _, token := range tools.Cfg.Campus.Tokens {
			events, deleted, err = getEventsForCourse(course.TUMOnlineIdentifier, token)
			if err == nil {
				break
			}
		}
		ids := make([]uint, len(deleted))
		for i := range deleted {
			ids[i] = deleted[i].SingleEventID
		}
		dao.DeleteStreamsWithTumID(ids)
		for _, event := range events {
			stream, err := dao.GetStreamByTumOnlineID(context.Background(), event.SingleEventID)
			if err != nil { // Lecture does not exist yet
				log.Info("Adding course")
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
			} else {
				stream.RoomCode = event.RoomCode
				stream.RoomName = event.RoomName
				stream.Start = event.Start
				stream.End = event.End
				stream.EventTypeName = event.SingleEventTypeName
			}
		}
		err = dao.UpdateCourse(context.Background(), course)
		if err != nil {
			log.WithError(err).WithField("CourseID", course.ID).Warn("Can't update course")
		}
	}
}

func GetCurrentSemester() (year int, term string) {
	var curTerm string
	var curYear int
	if time.Now().Month() >= 4 && time.Now().Month() < 10 {
		curTerm = "S"
		curYear = time.Now().Year()
	} else {
		curTerm = "W"
		if time.Now().Month() >= 10 {
			curYear = time.Now().Year()
		} else {
			curYear = time.Now().Year() - 1
		}
	}

	return curYear, curTerm
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
