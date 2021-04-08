package tum

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"fmt"
	"github.com/antchfx/xmlquery"
	"log"
)

func FindStudentsForCourses(courses []model.Course) {
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
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/cdm/course/students/xml?token=%v&courseID=%v", tools.Cfg.CampusBase, tools.Cfg.CampusToken, courseID))
	if err != nil {
		log.Printf("couldn't load TUMOnline xml: %v\n", err)
		return []string{}, err
	}
	res, err := xmlquery.QueryAll(doc, "//person")
	if err != nil {
		log.Printf("Malformed TUMOnline xml: %v\n", err)
		return []string{}, err
	}
	ids := make([]string, len(res))
	for i := range res {
		ids[i] = res[i].SelectAttr("ident")
	}
	return ids, nil
}
