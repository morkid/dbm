package dbm

import "gorm.io/gorm"

// RegisterDriver registers a database driver
func RegisterDriver(dialect string, builder ConnectionBuilder) {
	drivers[dialect] = builder
}

// GetDriver returns a registered driver by dialect name
func GetDriver(dialect string) (ConnectionBuilder, bool) {
	b, ok := drivers[dialect]
	return b, ok
}

// ConnectionBuilder is a builder for database connections
type ConnectionBuilder struct {
	BuildDSN      func(config Config) string
	Open          func(dsn string) gorm.Dialector
	DefaultConfig Config
}

var drivers = map[string]ConnectionBuilder{}
