package apiv2

import (
	"context"
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// infoPageIDs maps a route name to its row. Keyed on fixed ids, not the `name`
// column, which an administrator edits: a rename would otherwise 404 the page.
var infoPageIDs = map[string]uint{
	"privacy": 1,
	"imprint": 2,
	"about":   3,
}

// GetInfoPage returns an info page as HTML, rendered and sanitised here so that
// bluemonday stays the only thing deciding what may appear in the document.
func (a *API) GetInfoPage(ctx context.Context, req *protobuf.GetInfoPageRequest) (*protobuf.GetInfoPageResponse, error) {
	id, ok := infoPageIDs[req.GetName()]
	if !ok {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such info page"))
	}

	// GetById uses Find, so an unseeded row comes back zero-valued rather than as an
	// error: the page reads as empty, exactly as the template rendered it.
	page, err := a.dao.InfoPageDao.GetById(id)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.GetInfoPageResponse{
		Name:    req.GetName(),
		Content: string(page.Render()),
	}, nil
}
