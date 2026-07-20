package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEnsureUsersPhoneSchemaIsIdempotentAndCreatesUniqueIndex(t *testing.T) {
	dsn := fmt.Sprintf("file:phone_schema_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	originalDB := DB
	originalMain := common.MainDatabaseType()
	originalLog := common.LogDatabaseType()
	DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = originalDB
		common.SetDatabaseTypes(originalMain, originalLog)
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	// Simulate an existing users table without phone / with alternate unique names.
	require.NoError(t, db.Exec(`
CREATE TABLE users (
  id integer primary key,
  username text,
  password text not null
)`).Error)

	// First run adds column + canonical unique index.
	require.NoError(t, ensureUsersPhoneSchema())
	require.True(t, DB.Migrator().HasColumn(&User{}, "Phone"))

	var cnt int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(1) FROM sqlite_master WHERE type='index' AND name=?`,
		usersPhoneUniqueIndex,
	).Scan(&cnt).Error)
	require.EqualValues(t, 1, cnt)

	// Alternate names should be dropped if present; re-run must stay green.
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uni_users_phone ON users(phone)`).Error)
	require.NoError(t, ensureUsersPhoneSchema())
	require.NoError(t, db.Raw(
		`SELECT COUNT(1) FROM sqlite_master WHERE type='index' AND name=?`,
		usersPhoneUniqueIndex,
	).Scan(&cnt).Error)
	require.EqualValues(t, 1, cnt)
	require.NoError(t, db.Raw(
		`SELECT COUNT(1) FROM sqlite_master WHERE type='index' AND name='uni_users_phone'`,
	).Scan(&cnt).Error)
	require.EqualValues(t, 0, cnt)

	// Second ensure is a no-op.
	require.NoError(t, ensureUsersPhoneSchema())
}
