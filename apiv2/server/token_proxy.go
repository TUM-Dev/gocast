package apiv2

import (
	"net/http"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
)

// handleTokenProxy is called by rtmp-proxy/publish.sh (through nginx) to get the stream
// key of the next stream of the lecturer given a lecturer token. Issuing and revoking
// tokens is token_admin.go; this serves the RTMP proxy rather than a browser.
//
//	Proxy receives: rtmp://proxy.example.com/<lecturer-token>
//				or: rtmp://proxy.example.com/<lecturer-token>?slug=ABC-123 <-- optional slug parameter in case the lecturer is streaming multiple courses simultaneously
//
//	Proxy returns:  rtmp://ingest.example.com/ABC-123?secret=610f609e4a2c43ac8a6d648177472b17
func (a *API) handleTokenProxy(w http.ResponseWriter, r *http.Request) {
	t := r.PathValue("token")
	// Optional slug parameter to get the stream key of a specific course (in case the lecturer is streaming multiple courses simultaneously)
	slug := r.URL.Query().Get("slug")

	token, err := a.dao.TokenDao.GetToken(t)
	if err != nil {
		writeLegacyError(w, http.StatusBadRequest, "invalid token", nil)
		return
	}

	// Only tokens of type lecturer are allowed to start streaming
	if token.Scope != model.TokenScopeLecturer {
		writeLegacyError(w, http.StatusUnauthorized, "invalid scope", nil)
		return
	}

	// Get user and check if he has the right to start a stream
	user, err := a.dao.UsersDao.GetUserByID(r.Context(), token.UserID)
	if err != nil {
		writeLegacyError(w, http.StatusInternalServerError, "could not get user", err)
		return
	}
	if !user.Can(model.PermLecture) {
		writeLegacyError(w, http.StatusUnauthorized, "user is not a lecturer or admin", nil)
		return
	}

	// Find current/next stream and course of which the user is a lecturer
	year, term := tum.GetCurrentSemester()
	streamKey, courseSlug, err := a.dao.StreamsDao.GetSoonStartingStreamInfo(&user, slug, year, term)
	if err != nil || streamKey == "" || courseSlug == "" {
		writeLegacyError(w, http.StatusNotFound, "no stream found", err)
		return
	}

	tools.WriteJSON(w, http.StatusOK, map[string]any{"url": "" + tools.Cfg.IngestBase + courseSlug + "?secret=" + streamKey + "/" + courseSlug})
}
