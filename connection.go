package dbm

import "gorm.io/gorm"

// Connection is an interface for database connections
type Connection interface {
	Register(name string, config Config, autoConnect ...bool) error
	Get(name string) (*gorm.DB, error)
	Connect(name string, override ...bool) (*gorm.DB, error)
	Migrate(name string) error
	GetDefault() *gorm.DB
	SetDefault(name string)
}
