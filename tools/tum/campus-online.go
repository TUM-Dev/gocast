package tum

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"errors"
	"fmt"
	"github.com/antchfx/xmlquery"
	"log"
)

func GetCourseInformation(courseID string) (CourseInfo, error) {
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/course/students/xml?token=%v&courseID=%v", tools.Cfg.CampusBase, tools.Cfg.CampusToken, courseID))
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
	courseInfo.TeachingTerm = xmlquery.FindOne(doc, "//teachingTerm").InnerText()
	courseInfo.NumberAttendees = len(xmlquery.Find(doc, "//personID"))
	return courseInfo, nil
}

func FindStudentsForAllCourses() {
	courses, err := dao.GetAllCoursesWithTUMID(context.Background())
	if err != nil {
		log.Printf("Could not get courses with TUM online identifier: %v", err)
		return
	}
	for i := range courses {
		studentIDs, err := findStudentsForCourse(courses[i].TUMOnlineIdentifier)
		if err != nil {
			log.Printf("Could not get Students for course with id %v: %v\n", courses[i].TUMOnlineIdentifier, err)
			break
		}
		students := make([]model.Student, len(studentIDs))
		for j := range students {
			students[j] = model.Student{ID: studentIDs[j]}
		}
		courses[i].Students = students
	}
	dao.UpdateCourses(context.Background(), courses)
}

/**
 * scans the CampusOnline API for enrolled students in one course
 */
func findStudentsForCourse(courseID string) (obfuscatedIDs []string, err error) {
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/course/students/xml?token=%v&courseID=%v", tools.Cfg.CampusBase, tools.Cfg.CampusToken, courseID))
	if err != nil {
		log.Printf("couldn't load TUMOnline xml: %v\n", err)
		return []string{}, err
	}
	res, err := xmlquery.QueryAll(doc, "//personID")
	if err != nil {
		log.Printf("Malformed TUMOnline xml: %v\n", err)
		return []string{}, err
	}
	ids := make([]string, len(res))
	for i := range res {
		ids[i] = res[i].InnerText()
	}
	return ids, nil
}

type CourseInfo struct {
	CourseName      string `json:"courseName"`
	TumOnlineId     string `json:"tumOnlineID"`
	NumberAttendees int    `json:"numberAttendees"`
	TeachingTerm    string `json:"teachingTerm"`
}
