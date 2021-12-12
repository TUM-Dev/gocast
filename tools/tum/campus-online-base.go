package tum

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"errors"
	"fmt"
	"github.com/antchfx/xmlquery"
	log "github.com/sirupsen/logrus"
)

func GetCourseInformation(courseID string, token string) (CourseInfo, error) {
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/cdm/course/students/xml?token=%v&courseID=%v", tools.Cfg.Campus.Base, token, courseID))
	if err != nil {
		return CourseInfo{}, fmt.Errorf("GetCourseInformation: Can't LoadURL: %v", err)
	}
	var isError = len(xmlquery.Find(doc, "//Error")) != 0
	if isError {
		return CourseInfo{}, errors.New("course not found")
	}
	var courseInfo CourseInfo
	courseInfo.TumOnlineId = courseID
	courseInfo.CourseName = xmlquery.FindOne(doc, "//courseName/text").InnerText()
	// turns Sommersemester 2020 into SoSe2020
	courseInfo.TeachingTerm = xmlquery.FindOne(doc, "//teachingTerm").InnerText()
	courseInfo.NumberAttendees = len(xmlquery.Find(doc, "//personID"))
	return courseInfo, nil
}

func FetchCourses() {
	y, t := GetCurrentSemester()
	courses, err := dao.GetAllCoursesWithTUMIDForSemester(context.Background(), y, t)
	if err != nil {
		log.WithError(err).Error("Could not get courses with TUM online identifier:", err)
		return
	}
	FindStudentsForCourses(courses)
	//GetEventsForCourses(courses)
}

type CourseInfo struct {
	CourseName       string `json:"courseName"`
	TumOnlineId      string `json:"tumOnlineID"`
	NumberAttendees  int    `json:"numberAttendees"`
	TeachingTerm     string `json:"teachingTerm"`
	TeachingTermFull string `json:"teachingTermFull"`
}
