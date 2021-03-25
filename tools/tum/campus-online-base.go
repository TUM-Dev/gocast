package tum

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"errors"
	"fmt"
	"github.com/antchfx/xmlquery"
	"log"
)

func GetCourseInformation(courseID string) (CourseInfo, error) {
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/cdm/course/students/xml?token=%v&courseID=%v", tools.Cfg.CampusBase, tools.Cfg.CampusToken, courseID))
	if err != nil {
		log.Printf("couldn't load TUMOnline xml: %v\n", err)
		return CourseInfo{}, err
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
	courses, err := dao.GetAllCoursesWithTUMID(context.Background())
	if err != nil {
		log.Printf("Could not get courses with TUM online identifier: %v", err)
		return
	}
	findStudentsForAllCourses(courses)
	getEventsForCourses(courses)
}

type CourseInfo struct {
	CourseName       string `json:"courseName"`
	TumOnlineId      string `json:"tumOnlineID"`
	NumberAttendees  int    `json:"numberAttendees"`
	TeachingTerm     string `json:"teachingTerm"`
	TeachingTermFull string `json:"teachingTermFull"`
}
