package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func TestUnknownInfoPageIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	infoPageDao := mock_dao.NewMockInfoPageDao(ctrl)
	infoPageDao.EXPECT().GetBySlug("ws").Return(model.InfoPage{}, gorm.ErrRecordNotFound)

	router := tools.NewRouter()
	routes := mainRoutes{DaoWrapper: dao.DaoWrapper{InfoPageDao: infoPageDao}}
	router.HandleFunc("GET /{slug}", routes.infoPage)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
