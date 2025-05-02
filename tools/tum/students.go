package tum

import (
	"errors"
	"fmt"

	"github.com/antchfx/xmlquery"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
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
			logger.Error("FindStudentsForCourses: Can't get Students for course with id", "err", err, "TUMOnlineIdentifier", courses[i].TUMOnlineIdentifier)
			continue
		}
		err = usersDao.AddUsersToCourseByTUMIDs(studentIDs, courses[i].ID)
		if err != nil {
			logger.Error("FindStudentsForCourses: Can't add users to course", "err", err)
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
	ids := make([]string, 0, len(res))
	for _, r := range res {
		found := xmlquery.Find(r, "infoBlock/subBlock[@userDefined='attendance']")
		if len(found) == 1 && found[0].InnerText() == "J" {
			ids = append(ids, r.SelectAttr("ident"))
		}
	}
	return ids, nil
}
