package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMainDatabaseMigrationCreatesPermissionTables(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	previousMainDBType := common.MainDatabaseType()
	previousLogDBType := common.LogDatabaseType()
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDBType, previousLogDBType)
		initCol()
	})

	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "permission-migration.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})

	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()

	require.NoError(t, migrateDB())
	assert.True(t, DB.Migrator().HasTable(&CasbinRule{}))
	assert.True(t, DB.Migrator().HasTable(&AuthzRole{}))
	assert.True(t, DB.Migrator().HasTable(&UserManagementPermission{}))
}
