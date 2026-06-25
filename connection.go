package dbm

import "gorm.io/gorm"

// Connection is an interface for database connections
type Connection interface {
	// Register registers a database connection under the given name using
	// the provided config. The first registered connection is automatically
	// promoted to the default. When autoConnect is true, Register also
	// opens the connection immediately; otherwise the underlying handle is
	// opened lazily on the first call to Connect or Migrate. A second
	// Register with a name that is already known is a no-op.
	Register(name string, config Config, autoConnect ...bool) error

	// Get returns the GORM handle currently associated with the given
	// name. It returns an error when no connection has been registered
	// under that name.
	Get(name string) (*gorm.DB, error)

	// Connect opens the underlying database for the registered name and
	// returns its GORM handle. When override is false, the freshly opened
	// handle is used transiently and does not replace the stored
	// connection (used internally by Migrate); when override is true, or
	// when override is omitted entirely, the opened handle replaces the
	// one stored under the name.
	Connect(name string, override ...bool) (*gorm.DB, error)

	// Migrate runs gorm.AutoMigrate over the connection's MigrationItems
	// and applies the configured MigrationSeeds inside a single
	// transaction per seed. It opens a transient connection via
	// Connect(name, false) that is closed once migration completes.
	Migrate(name string) error

	// GetDefault returns the GORM handle for the connection currently
	// designated as default. It panics if no default has been set or if
	// the registered default name cannot be resolved.
	GetDefault() *gorm.DB

	// SetDefault designates the named connection as the one returned by
	// GetDefault. The named connection itself must already be registered.
	SetDefault(name string)

	// OnConnectionCreated registers a callback that is invoked once each
	// time a connection backed by this Connection is successfully opened.
	// The callback fires AFTER every gorm plugin on the connection has
	// been registered via dbConn.Use(plugin) and BEFORE the SQL connection
	// pool is configured. If a configured plugin fails to register, the
	// hook is silently suppressed so observers do not see a half-
	// configured dbConn.
	//
	// Per-instance hooks are scoped to the receiving Connection: a hook
	// registered on one mgr does not fire for connect calls on a different
	// mgr. The receiver is returned so calls can be chained, for example:
	//
	//	mgr.OnConnectionCreated(a).OnConnectionCreated(b)
	//
	// Hook registration is NOT safe for concurrent calls; serialize
	// OnConnectionCreated invocations with concurrent Connect() calls in
	// the calling code or supply your own synchronization.
	//
	// A nil callback is silently ignored (no error, no registration).
	OnConnectionCreated(callback ConnectionCallback) Connection
}
