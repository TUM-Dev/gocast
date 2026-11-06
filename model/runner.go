package model

import (
	"time"

	"gorm.io/gorm"
)

// Runner represents a runner handling streams, converting videos,
// extracting silence from audios, creating thumbnails, etc.
type Runner struct {
	// Hostname is the hostname of the runner
	Hostname string `gorm:"column:hostname;primaryKey;unique;not null"`
	// Port is the port, the runners gRPC server listens on.
	Port uint32 `gorm:"column:port;not null"`
	// LastSeen is the timestamp of the last successful heartbeat.
	// if the runner wasn't seen in more than 5 seconds, it's considered dead
	// and won't be assigned further jobs.
	LastSeen time.Time `gorm:"column:last_seen;"`
	// TimeOfRegister is the timestamp of the register message.
	TimeOfRegister time.Time `gorm:"column:time_of_register;"`
	// Draining is true if the runner is shutting down.
	// In this case, no further jobs will be assigned.
	Draining bool `gorm:"column:draining;not null;default:false"`
	// JobCount is the number of currently running jobs.
	// It's updated through heartbeats and used to select
	// the runner with the least workload for new jobs.
	JobCount uint64 `gorm:"column:job_count;not null;default:0"`
	// Version is the version of the runner.
	Version string `gorm:"column:version;not null;default:'dev'"`
	// CertSerial, CertFingerprint, CertNotAfter and CertNames describe the client certificate
	// the runner last connected with, for the administration page. They stay empty while mutual
	// TLS is off. The fingerprint is the SHA-256 of the DER encoding, in hex.
	CertSerial      string     `gorm:"column:cert_serial;not null;default:''"`
	CertFingerprint string     `gorm:"column:cert_fingerprint;not null;default:''"`
	CertNotAfter    *time.Time `gorm:"column:cert_not_after"`
	// CertNames holds the DNS names, comma separated.
	CertNames string `gorm:"column:cert_names;not null;default:''"`
}

// TableName returns the name of the table for the Runner model in the database.
func (*Runner) TableName() string {
	return "runners" // todo
}

// BeforeCreate returns an error, if Runner r is invalid
func (r *Runner) BeforeCreate(tx *gorm.DB) error {
	r.LastSeen = time.Now()
	return nil
}

// AfterFind runs after the Runner is fetched from the db
func (r *Runner) AfterFind(tx *gorm.DB) (err error) {
	// this method currently is a noop
	return nil
}

// Alive returns true if the runner is alive
func (r *Runner) Alive() bool {
	return r.LastSeen.Add(5 * time.Second).After(time.Now())
}
