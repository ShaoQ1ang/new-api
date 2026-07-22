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

func MainDatabaseType() DatabaseType {
	return mainDatabaseType
}

func LogDatabaseType() DatabaseType {
	return logDatabaseType
}

func SetMainDatabaseType(databaseType DatabaseType) {
	mainDatabaseType = databaseType
	refreshLegacyDBFlags()
}

func SetLogDatabaseType(databaseType DatabaseType) {
	logDatabaseType = databaseType
}

func SetDatabaseTypes(mainType DatabaseType, logType DatabaseType) {
	mainDatabaseType = mainType
	logDatabaseType = logType
	refreshLegacyDBFlags()
}

func UsingMainDatabase(databaseType DatabaseType) bool {
	return mainDatabaseType == databaseType
}

func UsingLogDatabase(databaseType DatabaseType) bool {
	return logDatabaseType == databaseType
}

var SQLitePath = "one-api.db?_busy_timeout=30000"


// Legacy flags kept for product-line models/controllers.
// Prefer UsingMainDatabase(DatabaseType*) for new code.
var UsingSQLite bool = true
var UsingMySQL bool
var UsingPostgreSQL bool

func refreshLegacyDBFlags() {
	UsingSQLite = mainDatabaseType == DatabaseTypeSQLite
	UsingMySQL = mainDatabaseType == DatabaseTypeMySQL
	UsingPostgreSQL = mainDatabaseType == DatabaseTypePostgreSQL
}
