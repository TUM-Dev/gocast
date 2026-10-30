package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrate202610300DeletesOnlyAdminTokens(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE tokens (id integer primary key, token text, scope text, deleted_at datetime)").Error)
	require.NoError(t, db.Exec("INSERT INTO tokens (id, token, scope) VALUES (1, 'a', 'admin'), (2, 'l', 'lecturer')").Error)

	require.NoError(t, Migrate202610300().Migrate(db))

	var ids []int
	require.NoError(t, db.Raw("SELECT id FROM tokens").Scan(&ids).Error)
	assert.Equal(t, []int{2}, ids)
}

func TestMigrate202610300WithoutATokensTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	assert.NoError(t, Migrate202610300().Migrate(db))
}
