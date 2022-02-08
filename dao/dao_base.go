package dao

import (
	"context"
	"fmt"
	"gorm.io/gorm"
)

// BuildInfo is used to define the application build info, and inject values into via the build process.
type BuildInfo struct {

	// BuildDate date string of when build was performed filled in by -X compile flag
	BuildDate string

	// LatestCommit date string of when build was performed filled in by -X compile flag
	LatestCommit string

	// BuildNumber date string of when build was performed filled in by -X compile flag
	BuildNumber string

	// BuiltOnIP date string of when build was performed filled in by -X compile flag
	BuiltOnIP string

	// BuiltOnOs date string of when build was performed filled in by -X compile flag
	BuiltOnOs string

	// RuntimeVer date string of when build was performed filled in by -X compile flag
	RuntimeVer string
}

type LogSql func(ctx context.Context, sql string)

var (
	// ErrNotFound error when record not found
	ErrNotFound = fmt.Errorf("record Not Found")

	// ErrUnableToMarshalJSON error when json payload corrupt
	ErrUnableToMarshalJSON = fmt.Errorf("json payload corrupt")

	// ErrUpdateFailed error when update fails
	ErrUpdateFailed = fmt.Errorf("db update error")

	// ErrInsertFailed error when insert fails
	ErrInsertFailed = fmt.Errorf("db insert error")

	// ErrDeleteFailed error when delete fails
	ErrDeleteFailed = fmt.Errorf("db delete error")

	// ErrBadParams error when bad params passed in
	ErrBadParams = fmt.Errorf("bad params error")

	// DB reference to database
	DB *gorm.DB

	// AppBuildInfo reference to build info
	AppBuildInfo *BuildInfo

	Migrator = newMigrator()
)
