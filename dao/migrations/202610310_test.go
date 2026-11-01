package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrate202610310DropsShortLinks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE short_links (id integer primary key, link text, course_id integer)").Error)

	require.NoError(t, Migrate202610310().Migrate(db))

	assert.False(t, db.Migrator().HasTable("short_links"))
}

func TestMigrate202610310WithoutAShortLinksTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	assert.NoError(t, Migrate202610310().Migrate(db))
}
