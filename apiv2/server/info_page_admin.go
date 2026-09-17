package apiv2

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Every RPC here is gated on PermAdministerServer by its policy in services.go.

// slugPattern is kebab-case: lowercase letters, digits and single internal hyphens.
// The slug becomes a URL segment, so anything else risks an unreachable page instead
// of a clear error at creation time.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedInfoPageSlugs are top-level paths another handler already owns. Creating a
// page with one of these names would not break that route -- the router matches the
// static route first -- but the page itself would be silently unreachable, which is
// worse than refusing it up front.
var reservedInfoPageSlugs = map[string]bool{
	"admin": true, "login": true, "logout": true, "search": true,
	"healthcheck": true, "jwtpubkey": true, "edit-course": true,
	"public": true, "static": true, "spa-assets": true, "service-worker.js": true,
}

// ListInfoPagesAdmin lists every info page with its raw content, for editing.
func (a *API) ListInfoPagesAdmin(ctx context.Context, req *emptypb.Empty) (*protobuf.ListInfoPagesAdminResponse, error) {
	pages, err := a.dao.InfoPageDao.GetAll()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.InfoPage, 0, len(pages))
	for _, page := range pages {
		out = append(out, infoPageMessage(page))
	}

	return &protobuf.ListInfoPagesAdminResponse{Pages: out}, nil
}

// CreateInfoPage creates a page reachable at /{slug} immediately, without a redeploy.
func (a *API) CreateInfoPage(ctx context.Context, req *protobuf.CreateInfoPageRequest) (*protobuf.InfoPage, error) {
	slug := strings.TrimSpace(req.GetSlug())
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a title is required"))
	}
	if !slugPattern.MatchString(slug) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New(
			"the slug must be lowercase letters, digits and hyphens, e.g. 'terms-of-use'"))
	}
	if reservedInfoPageSlugs[slug] {
		return nil, e.WithStatus(http.StatusConflict, errors.New("that slug is used by another page of the site"))
	}

	if _, err := a.dao.InfoPageDao.GetBySlug(slug); err == nil {
		return nil, e.WithStatus(http.StatusConflict, errors.New("a page with that slug already exists"))
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	page := model.InfoPage{
		Slug:       slug,
		Name:       name,
		RawContent: req.GetRawContent(),
		Type:       model.INFOPAGE_MARKDOWN,
	}
	if err := a.dao.InfoPageDao.New(&page); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return infoPageMessage(page), nil
}

// UpdateInfoPage replaces an info page's slug, title and content.
func (a *API) UpdateInfoPage(ctx context.Context, req *protobuf.UpdateInfoPageRequest) (*protobuf.InfoPage, error) {
	slug := strings.TrimSpace(req.GetSlug())
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a title is required"))
	}
	if !slugPattern.MatchString(slug) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New(
			"the slug must be lowercase letters, digits and hyphens, e.g. 'terms-of-use'"))
	}
	if reservedInfoPageSlugs[slug] {
		return nil, e.WithStatus(http.StatusConflict, errors.New("that slug is used by another page of the site"))
	}

	id := uint(req.GetId())
	existing, err := a.dao.InfoPageDao.GetById(id)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if existing.ID == 0 {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such info page"))
	}

	if bySlug, err := a.dao.InfoPageDao.GetBySlug(slug); err == nil && bySlug.ID != id {
		return nil, e.WithStatus(http.StatusConflict, errors.New("a page with that slug already exists"))
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	page := model.InfoPage{Slug: slug, Name: name, RawContent: req.GetRawContent()}
	if err := a.dao.InfoPageDao.Update(id, &page); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	page.ID = id
	return infoPageMessage(page), nil
}

// DeleteInfoPage deletes an info page; its route stops resolving immediately.
func (a *API) DeleteInfoPage(ctx context.Context, req *protobuf.DeleteInfoPageRequest) (*emptypb.Empty, error) {
	if err := a.dao.InfoPageDao.Delete(uint(req.GetId())); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

func infoPageMessage(page model.InfoPage) *protobuf.InfoPage {
	return &protobuf.InfoPage{
		Id:         uint32(page.ID),
		Slug:       page.Slug,
		Name:       page.Name,
		RawContent: page.RawContent,
	}
}
