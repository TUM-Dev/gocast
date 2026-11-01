package web

import (
	"errors"
	"net/http"

	"gorm.io/gorm"
)

// infoPage serves an info page created after the three built-in ones, which cannot be
// given a route of their own at boot: it checks the info_pages table, so a page an
// administrator creates is reachable immediately, without a deploy. Any other
// single-segment path is the same 404 as everywhere else.
func (r mainRoutes) infoPage(w http.ResponseWriter, req *http.Request) {
	slug := req.PathValue("slug")
	if _, err := r.InfoPageDao.GetBySlug(slug); err == nil {
		serveSPAShell(w, req)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("can't look up info page", "err", err, "slug", slug)
	}

	serveNotFound(w, req)
}
