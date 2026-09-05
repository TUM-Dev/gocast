package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/matthiasreumann/gomino"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
)

type integrationErrorTemplate struct{}

func (integrationErrorTemplate) ExecuteTemplate(io.Writer, string, interface{}) error { return nil }

func IntegrationRouterWrapper(r *gin.Engine) {
	configIntegrationRouter(r, dao.DaoWrapper{})
}

func TestCreateIntegration(t *testing.T) {
	t.Run("responses", func(t *testing.T) {
		tools.SetTemplateExecutor(integrationErrorTemplate{})
		gomino.TestCases{
			"POST[lecturer]": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturer)),
				Body:         createIntegrationRequest{Name: "Course portal", ReturnURL: "https://portal.example/callback"},
				ExpectedCode: http.StatusForbidden,
			},
			"POST[invalid return URL]": {
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:        createIntegrationRequest{Name: "Course portal", ReturnURL: "http://portal.example/callback"},
				ExpectedResponse: gin.H{
					"status":  http.StatusBadRequest,
					"message": "Enter a name and an HTTPS return URL. HTTP is allowed for loopback development.",
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"POST[DAO error]": {
				Router: func(r *gin.Engine) {
					integrationDao := mock_dao.NewMockIntegrationDao(gomock.NewController(t))
					integrationDao.EXPECT().CreateIntegration(gomock.Any()).Return(errors.New("database unavailable"))
					configIntegrationRouter(r, dao.DaoWrapper{IntegrationDao: integrationDao})
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:        createIntegrationRequest{Name: "Course portal", ReturnURL: "https://portal.example/callback"},
				ExpectedResponse: gin.H{
					"status":  http.StatusInternalServerError,
					"message": "Could not create the integration.",
					"error":   "database unavailable",
				},
				ExpectedCode: http.StatusInternalServerError,
			},
		}.
			Router(IntegrationRouterWrapper).
			Method(http.MethodPost).
			Url("/api/integrations").
			Run(t, testutils.Equal)
	})

	t.Run("success", func(t *testing.T) {
		var created *model.Integration
		gomino.TestCases{
			"POST[success]": {
				Router: func(r *gin.Engine) {
					integrationDao := mock_dao.NewMockIntegrationDao(gomock.NewController(t))
					integrationDao.EXPECT().CreateIntegration(gomock.Any()).DoAndReturn(func(integration *model.Integration) error {
						integration.ID = 7
						created = integration
						return nil
					})
					configIntegrationRouter(r, dao.DaoWrapper{IntegrationDao: integrationDao})
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:             createIntegrationRequest{Name: "Course portal", ReturnURL: "https://portal.example/callback"},
				ExpectedCode:     http.StatusCreated,
				ExpectedResponse: []byte{},
			},
		}.
			Method(http.MethodPost).
			Url("/api/integrations").
			Run(t, func(t *testing.T, expected, actual interface{}) {
				body, isBody := actual.([]byte)
				if !isBody {
					testutils.Equal(t, expected, actual)
					return
				}
				var response struct {
					ID        uint   `json:"id"`
					Name      string `json:"name"`
					ReturnURL string `json:"returnUrl"`
					APIKey    string `json:"apiKey"`
				}
				require.NoError(t, json.Unmarshal(body, &response))
				rawKey, err := base64.RawURLEncoding.DecodeString(response.APIKey)
				require.NoError(t, err)
				wantHash := sha256.Sum256(rawKey)
				require.NotNil(t, created)
				assert.Equal(t, uint(7), response.ID)
				assert.Equal(t, "Course portal", response.Name)
				assert.Equal(t, "https://portal.example/callback", response.ReturnURL)
				assert.Len(t, rawKey, 32)
				assert.Equal(t, wantHash[:], created.APIKeyHash)
			})
	})
}

func TestValidIntegrationReturnURL(t *testing.T) {
	tests := map[string]bool{
		"https://portal.example/callback":          true,
		"http://localhost:8080/callback":           true,
		"http://127.0.0.1/callback":                true,
		"http://portal.example/callback":           false,
		"https://portal.example/#fragment":         false,
		"https://portal.example/?next=/x#fragment": false,
		"https://:443/callback":                    false,
		"//portal.example/callback":                false,
	}
	for raw, want := range tests {
		assert.Equal(t, want, validIntegrationReturnURL(raw), raw)
	}
}

func TestRotateIntegrationKey(t *testing.T) {
	var storedHash []byte
	gomino.TestCases{
		"POST[success]": {
			Router: func(r *gin.Engine) {
				integrationDao := mock_dao.NewMockIntegrationDao(gomock.NewController(t))
				integrationDao.EXPECT().SetIntegrationAPIKey(uint(7), gomock.Any()).DoAndReturn(func(_ uint, hash []byte) error {
					storedHash = bytes.Clone(hash)
					return nil
				})
				configIntegrationRouter(r, dao.DaoWrapper{IntegrationDao: integrationDao})
			},
			Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
			ExpectedCode:     http.StatusOK,
			ExpectedResponse: []byte{},
		},
	}.
		Method(http.MethodPost).
		Url("/api/integrations/7/key").
		Run(t, func(t *testing.T, expected, actual interface{}) {
			body, isBody := actual.([]byte)
			if !isBody {
				testutils.Equal(t, expected, actual)
				return
			}
			var response struct {
				APIKey string `json:"apiKey"`
			}
			require.NoError(t, json.Unmarshal(body, &response))
			rawKey, err := base64.RawURLEncoding.DecodeString(response.APIKey)
			require.NoError(t, err)
			wantHash := sha256.Sum256(rawKey)
			assert.Len(t, rawKey, 32)
			assert.Equal(t, wantHash[:], storedHash)
		})
}

func TestRevokeIntegrationKey(t *testing.T) {
	gomino.TestCases{
		"DELETE[success]": {
			Router: func(r *gin.Engine) {
				integrationDao := mock_dao.NewMockIntegrationDao(gomock.NewController(t))
				integrationDao.EXPECT().SetIntegrationAPIKey(uint(7), nil).Return(nil)
				configIntegrationRouter(r, dao.DaoWrapper{IntegrationDao: integrationDao})
			},
			Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
			ExpectedCode: http.StatusNoContent,
		},
	}.
		Method(http.MethodDelete).
		Url("/api/integrations/7/key").
		Run(t, testutils.Equal)
}
