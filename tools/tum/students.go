package tum

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"fmt"
	"github.com/antchfx/xmlquery"
	log "github.com/sirupsen/logrus"
)

func FindStudentsForCourses(courses []model.Course) {
	for i := range courses {
		studentIDs, err := findStudentsForCourse(courses[i].TUMOnlineIdentifier)
		if err != nil {
			log.WithError(err).WithField("TUMOnlineIdentifier", courses[i].TUMOnlineIdentifier).Error("FindStudentsForCourses: Can't get Students for course with id")
			continue
		}
		err = dao.AddUsersToCourseByTUMIDs(studentIDs, courses[i].ID)
		if err != nil {
			log.WithError(err).Error("FindStudentsForCourses: Can't add users to course")
		}
	}
}

/**
 * scans the CampusOnline API for enrolled students in one course
 */
func findStudentsForCourse(courseID string) (obfuscatedIDs []string, err error) {
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/cdm/course/students/xml?token=%v&courseID=%v", tools.Cfg.CampusBase, tools.Cfg.CampusToken, courseID))
	if err != nil {
		return []string{}, fmt.Errorf("findStudentsForCourse: couldn't load TUMOnline xml: %v", err)
	}
	res, err := xmlquery.QueryAll(doc, "//person")
	if err != nil {
		return []string{}, fmt.Errorf("findStudentsForCourse: Malformed TUMOnline xml: %v", err)
	}
	ids := make([]string, len(res))
	for i := range res {
		ids[i] = res[i].SelectAttr("ident")
	}
	return ids, nil
}
