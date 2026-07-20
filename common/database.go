package common

type DatabaseType string

const (
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
	DatabaseTypePostgreSQL DatabaseType = "postgres"
	DatabaseTypeClickHouse DatabaseType = "clickhouse"
)

var mainDatabaseType = DatabaseTypeSQLite
var logDatabaseType = DatabaseTypeSQLite

// Legacy flags kept for deploy-dev business code and tests.
// Prefer UsingMainDatabase / UsingLogDatabase for new code.
var UsingSQLite = true
var UsingPostgreSQL = false
var UsingMySQL = false
var UsingClickHouse = false
var LogSqlType = DatabaseTypeSQLite

func MainDatabaseType() DatabaseType {
	return mainDatabaseType
}

func LogDatabaseType() DatabaseType {
	return logDatabaseType
}

func SetMainDatabaseType(databaseType DatabaseType) {
	mainDatabaseType = databaseType
	syncMainDatabaseFlags()
}

func SetLogDatabaseType(databaseType DatabaseType) {
	logDatabaseType = databaseType
	LogSqlType = databaseType
	syncLogDatabaseFlags()
}

func SetDatabaseTypes(mainType DatabaseType, logType DatabaseType) {
	SetMainDatabaseType(mainType)
	SetLogDatabaseType(logType)
}

func UsingMainDatabase(databaseType DatabaseType) bool {
	return mainDatabaseType == databaseType
}

func UsingLogDatabase(databaseType DatabaseType) bool {
	return logDatabaseType == databaseType
}

func syncMainDatabaseFlags() {
	UsingSQLite = mainDatabaseType == DatabaseTypeSQLite
	UsingPostgreSQL = mainDatabaseType == DatabaseTypePostgreSQL
	UsingMySQL = mainDatabaseType == DatabaseTypeMySQL
	// ClickHouse is only used as a log database in current architecture.
}

func syncLogDatabaseFlags() {
	UsingClickHouse = logDatabaseType == DatabaseTypeClickHouse
	LogSqlType = logDatabaseType
}

var SQLitePath = "one-api.db?_busy_timeout=30000"
