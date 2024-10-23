package bot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/TUM-Dev/gocast/tools"
	"github.com/getsentry/sentry-go"
)

// Matrix strategy
type Matrix struct{}

// matrixMessage represents a Matrix message event that includes html formatting as specified
// in https://spec.matrix.org/v1.2/client-server-api/#mroommessage-msgtypes
type matrixMessage struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	Format        string `json:"format"`
	FormattedBody string `json:"formatted_body"`
}

// login represents a login request that is used for authentication as specified
// in https://spec.matrix.org/v1.2/client-server-api/#login
type login struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// loginResponse is the response for a login request
type loginResponse struct {
	UserId      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	HomeServer  string `json:"home_server"`
	DeviceId    string `json:"device_id"`
	WellKnown   struct {
		MHomeserver struct {
			BaseUrl string `json:"base_url"`
		} `json:"m.homeserver"`
	} `json:"well_known"`
}

const (
	// Maximum transaction ID
	maxID = 10000

	// Suffixes and Prefixes for sending a message to a room
	accessTokenSuffix = "?access_token="
	loginSuffix       = "login"
	roomSuffix        = "rooms/"
	roomMsgPrefix     = "/send/m.room.message/"

	// Constants describing the messages itself
	msgType     = "m.text"
	msgFormat   = "org.matrix.custom.html"
	loginMethod = "m.login.password"

	contentType = "application/json"
)

func (m *Matrix) getClientUrl() string {
	return "https://" + tools.Cfg.Alerts.Matrix.Homeserver + "/_matrix/client/r0/"
}

// SendBotMessage sends a formatted message to a matrix room with id roomID.
func (m *Matrix) SendBotMessage(message Message) error {
	err := m.sendMessageToRoom(tools.Cfg.Alerts.Matrix.LogRoomID, message)
	if err != nil {
		logger.Error("Failed to send message to matrix log room", "err", err)
		sentry.CaptureException(err)
		return err
	}
	if message.Prio {
		err = m.sendMessageToRoom(tools.Cfg.Alerts.Matrix.AlertRoomID, message)
		if err != nil {
			logger.Error("Failed to send message to matrix alert room", "err", err)
			sentry.CaptureException(err)
		}
	}
	return err
}

func (m *Matrix) sendMessageToRoom(roomID string, message Message) error {
	id := strconv.Itoa(rand.Intn(maxID)) // transaction id
	authToken, err := m.getAuthToken()
	if err != nil {
		return err
	}
	if authToken == "" {
		return errors.New("authentication failed, could not get token")
	}

	roomMessageSuffix := roomMsgPrefix + id + accessTokenSuffix
	url := m.getClientUrl() + roomSuffix + roomID + roomMessageSuffix + authToken
	matrixMessage := matrixMessage{
		MsgType:       msgType,
		Body:          message.Text,
		Format:        msgFormat,
		FormattedBody: getFormattedMessageText(message.Text),
	}
	matrixMessageJSON, err := json.Marshal(matrixMessage)
	if err != nil {
		return err
	}
	err = m.sendMessageRequest(url, bytes.NewBuffer(matrixMessageJSON))
	return err
}

// sendMessageRequest sends a PUT request to url with a given body.
func (m *Matrix) sendMessageRequest(url string, body io.Reader) error {
	client := &http.Client{}
	request, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(fmt.Sprintf("received status code %d instead of %d.", response.StatusCode, http.StatusOK))
	}
	return err
}

// getAuthToken retrieves a single use token for the next message sent to the server.
func (m *Matrix) getAuthToken() (string, error) {
	login := login{
		Type:     loginMethod,
		User:     tools.Cfg.Alerts.Matrix.Username,
		Password: tools.Cfg.Alerts.Matrix.Password,
	}
	loginRequest, err := json.Marshal(login)
	if err != nil {
		return "", err
	}
	response, err := http.Post(m.getClientUrl()+loginSuffix, contentType, bytes.NewBuffer(loginRequest))
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf(fmt.Sprintf("received status code %d instead of %d.", response.StatusCode, http.StatusOK))
	}
	loginResponse := loginResponse{}
	err = json.NewDecoder(response.Body).Decode(&loginResponse)
	if err != nil {
		return "", err
	}
	return loginResponse.AccessToken, err
}
