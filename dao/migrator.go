package dao

import (
	"TUM-Live/dao/migrations"
	"github.com/go-gormigrate/gormigrate/v2"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type migrator struct {
	migrationsBeforeAutoMigrate []*gormigrate.Migration
	migrationsAfterAutoMigrate  []*gormigrate.Migration
}

// RunBefore executes migrations before the auto-migration
func (m migrator) RunBefore(db *gorm.DB) error {
	log.Println("Running migrations before auto-migration")
	mig := gormigrate.New(db, gormigrate.DefaultOptions, m.migrationsBeforeAutoMigrate)
	return mig.Migrate()
}

// RunAfter executes migrations after the auto-migration
func (m migrator) RunAfter(db *gorm.DB) error {
	/*mig := gormigrate.New(db, gormigrate.DefaultOptions, m.migrationsAfterAutoMigrate)
	return mig.Migrate()*/ // comment in when needed
	return nil
}

func newMigrator() *migrator {
	return &migrator{
		migrationsBeforeAutoMigrate: []*gormigrate.Migration{
			migrations.Migrate202201280(),
		},
		migrationsAfterAutoMigrate: []*gormigrate.Migration{},
	}
}
