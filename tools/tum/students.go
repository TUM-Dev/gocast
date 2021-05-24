package tum

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"fmt"
	"github.com/antchfx/xmlquery"
	"github.com/getsentry/sentry-go"
	"log"
)

func FindStudentsForCourses(courses []model.Course) {
	for i := range courses {
		studentIDs, err := findStudentsForCourse(courses[i].TUMOnlineIdentifier)
		if err != nil {
			log.Printf("Could not get Students for course with id %v: %v\n", courses[i].TUMOnlineIdentifier, err)
			sentry.CaptureException(err)
			continue
		}
		err = dao.AddUsersToCourseByTUMIDs(studentIDs, courses[i].ID)
		if err != nil {
			log.Printf("%v", err)
			sentry.CaptureException(err)
		}
	}
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
