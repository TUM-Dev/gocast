package tum

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/TUM-Dev/CampusProxy/client"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model/search"
	"github.com/TUM-Dev/gocast/tools"
)

// PrefetchCourses loads all courses from tumonline, so we can use them in the course creation from search
func PrefetchCourses(dao dao.DaoWrapper) func() {
	return func() {
		client, err := tools.Cfg.GetMeiliClient()
		if err != nil {
			logger.Info("Skipping course prefetching, reason: ", "err", err)
			return
		}

		if tools.Cfg.Campus.CampusProxy == nil || tools.Cfg.Campus.RelevantOrgs == nil {
			return
		}
		var res []*search.PrefetchedCourse
		for _, org := range *tools.Cfg.Campus.RelevantOrgs {
			r, err := getCoursesForOrg(org, 0)
			if err != nil {
				logger.Error("Error getting courses for organisation "+org, "err", err)
			} else {
				res = append(res, r...)
			}
		}
		index := client.Index("PREFETCHED_COURSES")
		_, err = index.AddDocuments(&res, "courseID")
		logger.Info(string(rune(len(res))))
		if err != nil {
			logger.Error("issue adding documents to meili", "err", err)
		}
	}
}

// PrefetchCourses loads all courses from all schools known to gocast from tumonline, so we can use them in the course creation from search
func PrefetchAllCourses(dao dao.DaoWrapper) func() {
	return func() {
		client, err := tools.Cfg.GetMeiliClient()
		if err != nil {
			logger.Info("Skipping course prefetching, reason: ", "err", err)
			return
		}

		schools := dao.SchoolsDao.GetAll()

		if tools.Cfg.Campus.CampusProxy == nil || schools == nil {
			return
		}

		var res []*search.PrefetchedCourse
		for _, school := range schools {
			r, err := getCoursesForOrg(school.OrgId, school.ID)
			if err != nil {
				logger.Error("Error getting courses for organisation "+school.OrgSlug+" with ID "+school.OrgId, "err", err)
			} else {
				res = append(res, r...)
			}
		}
		index := client.Index("PREFETCHED_COURSES")
		_, err = index.AddDocuments(&res, "courseID")
		logger.Info(string(rune(len(res))))
		if err != nil {
			logger.Error("issue adding documents to meili", "err", err)
		}
	}
}

func getCoursesForOrg(org string, schoolID uint) ([]*search.PrefetchedCourse, error) {
	conf := client.NewConfiguration()
	conf.Host = "campus-proxy.mm.rbg.tum.de"
	conf.Scheme = "https"
	c := client.NewAPIClient(conf)
	courses, _, err := c.OrganizationApi.
		OrganizationCoursesGet(context.WithValue(context.Background(), client.ContextAPIKeys, map[string]client.APIKey{"ApiKeyAuth": {Key: tools.Cfg.Campus.Tokens[0]}})).
		IncludeChildren(true).
		OrgUnitID(org).
		Execute()
	if err.Error() != "" {
		return nil, fmt.Errorf("load Course: %v", err.Error())
	}
	var res []*search.PrefetchedCourse
	for _, c := range courses {
		t := "W"
		if strings.Contains(c.GetTeachingTerm(), "Sommer") {
			t = "S"
		}
		if !strings.Contains(c.GetTeachingTerm(), " ") {
			continue
		}
		y, err := strconv.Atoi(strings.Split(strings.Split(c.GetTeachingTerm(), " ")[1], "/")[0])
		if err != nil {
			continue
		}
		res = append(res, &search.PrefetchedCourse{
			Name:     c.CourseName.GetText(),
			CourseID: c.GetCourseId(),
			SchoolID: schoolID,
			Term:     t,
			Year:     y,
		})
	}
	return res, nil
}
