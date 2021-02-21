package api

import (
	"TUM-Live-Backend/dao"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func configGinOnboardingRouter(router gin.IRoutes) {
	router.GET("/onboarding/needsSetup", ConverHttprouterToGin(NeedsSetup))
}

/**
* This function is called when a user attempts to push a stream to the server.
* @w: response writer. Status code determines wether streaming is approved: 200 if yes, 402 otherwise.
* @r: request. Form if valid: POST /on_publish/app/kurs-key example: {/on_publish/eidi-3zt45z452h4754nj2q74}
 */
func NeedsSetup(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	needsSetup, err := dao.AreUsersEmpty(context.Background())
	if err != nil {
		w.WriteHeader(500)
		fmt.Printf("Couldn't query users: %v\n", err)
		return
	}
	writeJSON(context.Background(), w, needsSetupReply{NeedsSetup: needsSetup})
}

/* structs for sending to frontend: */

type needsSetupReply struct {
	NeedsSetup bool `json:"needsSetup"`
}
