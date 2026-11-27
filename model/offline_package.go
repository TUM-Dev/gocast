package model

import (
	"time"
)

// OfflinePackageState is where an offline package is in its life.
type OfflinePackageState string

const (
	// OfflinePreparing: a runner is making the package.
	OfflinePreparing OfflinePackageState = "preparing"
	// OfflineReady: the package is on the edge, waiting for the device to download it.
	OfflineReady OfflinePackageState = "ready"
	// OfflineDownloaded: the device has it, and the package is gone from the server. The
	// licence lives on, renewed by each sync.
	OfflineDownloaded OfflinePackageState = "downloaded"
	// OfflineFailed: the runner couldn't make the package.
	OfflineFailed OfflinePackageState = "failed"
	// OfflineRevoked: the viewer may no longer watch the lecture offline, or deleted it.
	OfflineRevoked OfflinePackageState = "revoked"
)

// OfflinePackage is one request to save a lecture for offline viewing, and the licence
// that came of it. Every request gets a package and a key of its own, so the rows also
// serve as the log of who saved what, and as the count behind the rate limits.
type OfflinePackage struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	UserID   uint                `gorm:"not null;index"`
	StreamID uint                `gorm:"not null;index"`
	Version  StreamVersion       `gorm:"not null"`
	State    OfflinePackageState `gorm:"not null;index"`

	// Key is the AES-128 key the package's segments are encrypted with. It is handed to
	// the device with the licence and kept here so a lost answer can be repeated.
	Key []byte `gorm:"not null"`

	SegmentCount uint32
	SizeBytes    uint64
	// Error is the runner's reason, meant for the viewer.
	Error string

	// ExpiresAt is when the device has to have renewed the licence; until it is first
	// handed out, it is zero.
	ExpiresAt time.Time
}

// Active reports whether the package still takes up space or is about to.
func (p OfflinePackage) Active() bool {
	return p.State == OfflinePreparing || p.State == OfflineReady
}
