package tum

import (
	"context"
	"fmt"
	"github.com/TUM-Dev/CampusProxy/client"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model/search"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

// PrefetchCourses loads all courses from tumonline, so we can use them in the course creation from search
func PrefetchCourses(dao dao.DaoWrapper) func() {
	return func() {
		client, err := tools.Cfg.GetMeiliClient()
		if err != nil {
			log.Info("Skipping course prefetching, reason: ", err)
			return
		}

		if tools.Cfg.Campus.CampusProxy == nil || tools.Cfg.Campus.RelevantOrgs == nil {
			return
		}
		var res []*search.PrefetchedCourse
		for _, org := range *tools.Cfg.Campus.RelevantOrgs {
			r, err := getCoursesForOrg(org)
			if err != nil {
				log.Error(err)
			} else {
				res = append(res, r...)
			}
		}
		index := client.Index("PREFETCHED_COURSES")
		_, err = index.AddDocuments(&res, "courseID")
		log.Info(len(res))
		if err != nil {
			log.WithError(err).Error("issue adding documents to meili")
		}
	}
}

func getCoursesForOrg(org string) ([]*search.PrefetchedCourse, error) {
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
			Term:     t,
			Year:     y,
		})
	}
	return res, nil
}
