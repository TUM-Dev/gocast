package tum

import (
	"context"
	"errors"
	"fmt"
	"github.com/antchfx/xmlquery"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
)

func FindStudentsForCourses(courses []model.Course, usersDao dao.UsersDao) {
	for i := range courses {
		var studentIDs []string
		var err error
		for _, token := range tools.Cfg.Campus.Tokens {
			studentIDs, err = findStudentsForCourse(courses[i].TUMOnlineIdentifier, token)
			if err == nil {
				break
			}
		}
		if err != nil {
			log.WithError(err).WithField("TUMOnlineIdentifier", courses[i].TUMOnlineIdentifier).Error("FindStudentsForCourses: Can't get Students for course with id")
			continue
		}
		err = usersDao.AddUsersToCourseByTUMIDs(context.Background(), studentIDs, courses[i].ID)
		if err != nil {
			log.WithError(err).Error("FindStudentsForCourses: Can't add users to course")
		}
	}
}

/**
 * scans the CampusOnline API for enrolled students in one course
 */
func findStudentsForCourse(courseID string, token string) (obfuscatedIDs []string, err error) {
	doc, err := xmlquery.LoadURL(fmt.Sprintf("%v/cdm/course/students/xml?token=%v&courseID=%v", tools.Cfg.Campus.Base, token, courseID))
	if err != nil {
		return []string{}, fmt.Errorf("findStudentsForCourse: couldn't load TUMOnline xml: %v", err)
	}
	if len(xmlquery.Find(doc, "//Error")) != 0 {
		return []string{}, errors.New("error found in xml")
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
