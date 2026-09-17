package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// GetInfoPage returns an info page as HTML, rendered and sanitised here so that
// bluemonday stays the only thing deciding what may appear in the document.
func (a *API) GetInfoPage(ctx context.Context, req *protobuf.GetInfoPageRequest) (*protobuf.GetInfoPageResponse, error) {
	page, err := a.dao.InfoPageDao.GetBySlug(req.GetName())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such info page"))
	} else if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.GetInfoPageResponse{
		Name:    req.GetName(),
		Content: string(page.Render()),
	}, nil
}

// ListInfoPages lists every info page's slug and title, without its content: enough
// for a client to know a page exists before asking getInfoPage for it.
func (a *API) ListInfoPages(ctx context.Context, req *emptypb.Empty) (*protobuf.ListInfoPagesResponse, error) {
	pages, err := a.dao.InfoPageDao.GetAll()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.InfoPageSummary, 0, len(pages))
	for _, page := range pages {
		out = append(out, &protobuf.InfoPageSummary{Slug: page.Slug, Name: page.Name})
	}

	return &protobuf.ListInfoPagesResponse{Pages: out}, nil
}
