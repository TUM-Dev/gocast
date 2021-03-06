package tools

import (
	"fmt"
	"github.com/antchfx/xmlquery"
	"log"
)

func FindStudentsForAllCourses(){
	// TODO: get CourseIDs from database, call findStudentsForCourse, save students to db
}

/**
 * scans the CampusOnline API for enrolled students in one course and stores them into the database
 */
func findStudentsForCourse(courseId string) {
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/course/students/xml?token=%v&courseID=%v", Cfg.CampusBase, Cfg.CampusToken, courseId))
	if err!=nil {
		log.Printf("couldn't load TUMOnline xml: %v\n", err)
		return
	}
	res, err := xmlquery.QueryAll(doc, "//personID")
	if err != nil {
		log.Printf("Malformed TUMOnline xml: %v\n", err)
		return
	}
	for i := range res {
		println(res[i].InnerText())
	}
}
