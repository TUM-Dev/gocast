package tum

import (
	"fmt"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/antchfx/xmlquery"
)

type Row struct {
	Ebene          int    `xml:"ebene"`
	Nr             int    `xml:"nr"`
	Parent         int    `xml:"parent"`
	ChildCnt       int    `xml:"child_cnt"`
	SortHierarchie int    `xml:"sort_hierarchie"`
	Kennung        string `xml:"kennung"`
	OrgTypName     string `xml:"org_typ_name"`
	OrgGruppeName  string `xml:"org_gruppe_name"`
	NameDe         string `xml:"name_de"`
	NameEn         string `xml:"name_en"`
}

type Data struct {
	Rows []Row `xml:"row"`
}

// Load TUMOnline XML tree (Reference: https://collab.dvb.bayern/display/tumonlineappdevndoc/orgBaum)
func LoadTUMOnlineOrgs(daoWrapper dao.DaoWrapper, token string) func() {
	return func() {
		// tree, err := xmlquery.LoadURL(fmt.Sprintf("%v/cdm/tree?token=%v", tools.Cfg.Campus.Base, token))
		tree, err := xmlquery.LoadURL(fmt.Sprintf("https://campus.tum.de/tumonline/wbservicesbasic.orgBaum?pToken=%v", token))
		if err != nil {
			logger.Error("Error loading XML from URL:", "err", err)
			return
		}

		orgNodes := xmlquery.Find(tree, "//row[org_typ_name='TUM School']")

		for _, node := range orgNodes {
			kennung := xmlquery.FindOne(node, "kennung").InnerText()
			nameEn := xmlquery.FindOne(node, "name_en").InnerText()
			nr := xmlquery.FindOne(node, "nr").InnerText()
			orgTypName := xmlquery.FindOne(node, "org_typ_name").InnerText()

			if orgTypName == "TUM School" {
				logger.Info("Loading organization", nr, kennung, orgTypName, nameEn)
				daoWrapper.OrganizationsDao.ImportOrganization(nr, kennung, orgTypName, nameEn)
			}
		}
		logger.Info("TUMOnline orgs loaded from URL.")
	}
}
