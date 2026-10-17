// Package uploads is how apiv2 reaches the runners that convert uploaded recordings.
package uploads

import (
	"context"
	"errors"

	apiv2 "github.com/TUM-Dev/gocast/apiv2/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
)

// Uploads reserves runners for uploads. Only a runner converts them, so Manager being
// nil means there is nobody to ask.
type Uploads struct {
	Manager *runner_manager.Manager
}

// Reserve picks a runner for one version of a stream's uploaded recording.
func (u Uploads) Reserve(ctx context.Context, streamID uint, version model.StreamVersion) (apiv2.UploadTarget, error) {
	if u.Manager == nil {
		return apiv2.UploadTarget{}, errors.New("no runner configured")
	}
	slot, err := u.Manager.RequestUpload(ctx, streamID, version)
	if err != nil {
		return apiv2.UploadTarget{}, err
	}
	return apiv2.UploadTarget{Host: slot.Host, Port: slot.Port, Token: slot.Token}, nil
}
