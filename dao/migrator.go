package dao

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/joschahenningsen/TUM-Live/dao/migrations"
	"gorm.io/gorm"
)

type migrator struct {
	migrationsBeforeAutoMigrate []*gormigrate.Migration
	migrationsAfterAutoMigrate  []*gormigrate.Migration
}

// RunBefore executes migrations before the auto-migration
func (m migrator) RunBefore(db *gorm.DB) error {
	// comment in when needed
	/*log.Println("Running migrations before auto-migration")
	mig := gormigrate.New(db, gormigrate.DefaultOptions, m.migrationsBeforeAutoMigrate)
	return mig.Migrate()*/
	return nil
}

// RunAfter executes migrations after the auto-migration
func (m migrator) RunAfter(db *gorm.DB) error {
	mig := gormigrate.New(db, gormigrate.DefaultOptions, m.migrationsAfterAutoMigrate)
	return mig.Migrate()
}

func newMigrator() *migrator {
	return &migrator{
		migrationsBeforeAutoMigrate: []*gormigrate.Migration{},
		migrationsAfterAutoMigrate: []*gormigrate.Migration{
			migrations.Migrate202210080(),
			migrations.Migrate202201280(),
			migrations.Migrate202207240(),
			migrations.Migrate202208110(),
			migrations.Migrate202210270(),
		},
	}
}
