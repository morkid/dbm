package dbm

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Config holds database connection configuration.
type Config struct {
	// ConnName is a unique identifier for this connection (e.g., "default", "replica").
	ConnName string `json:"conn_name,omitempty" example:"main"`

	// Type selects the database driver. Supported: "mysql", "postgres",
	// "sqlite", "sqlserver", "clickhouse".
	Type string `json:"type,omitempty" example:"mysql"`

	// Host is the database server hostname or IP address.
	Host string `json:"host,omitempty" example:"localhost"`

	// Port is the database server port (e.g., "3306" for MySQL).
	Port string `json:"port,omitempty" example:"3306"`

	// Name is the database name (or file path for SQLite).
	Name string `json:"name,omitempty" example:"test"`

	// User is the authentication username.
	User string `json:"user,omitempty" example:"root"`

	// Pass is the authentication password.
	Pass string `json:"pass,omitempty" example:"password"`

	// LogLevel controls GORM's log verbosity. Values: "silent", "error", "warn", "info".
	// Empty string defaults to "warn".
	LogLevel string `json:"log_level,omitempty" example:"warn"`

	// Logger sets a custom GORM logger. When set, LogLevel and LogSlowThreshold are ignored.
	Logger logger.Interface `json:"-"`

	// LogSlowThreshold is the slow query threshold in milliseconds. 0 means use default (200ms).
	LogSlowThreshold int `json:"log_slow_threshold,omitempty" example:"200"`

	// LogColorful enables colored GORM log output. Zero value means auto-detect based on stderr.
	LogColorful bool `json:"log_colorful,omitempty" example:"true"`

	// LogNotFound enables logging of "record not found" errors.
	LogNotFound bool `json:"log_not_found,omitempty" example:"true"`

	// Plugins registers GORM plugins (e.g., opentelemetry, prometheus).
	// Registered via dbConn.Use() on each Connect().
	Plugins []gorm.Plugin `json:"-"`

	// PrepareStmt enables prepared statement caching in GORM.
	PrepareStmt bool `json:"prepare_stmt,omitempty" example:"true"`

	// PrepareStmtMaxSize limits the prepared statement cache size. 0 means unlimited.
	PrepareStmtMaxSize int `json:"prepare_stmt_max_size,omitempty" example:"100"`

	// PrepareStmtTTL is the prepared statement TTL in seconds. 0 means no expiry.
	PrepareStmtTTL int `json:"prepare_stmt_ttl,omitempty" example:"300"`

	// Charset sets the connection charset (primarily for MySQL).
	Charset string `json:"charset,omitempty" example:"utf8mb4"`

	// Collation sets the MySQL connection collation (e.g. "utf8mb4_unicode_ci").
	// Appended as the `collation` DSN parameter.
	Collation string `json:"collation,omitempty" example:"utf8mb4_unicode_ci"`

	// DialTimeout is the MySQL connect timeout in seconds. Appended as
	// `connect_timeout=Ns` to the DSN. 0 leaves it unset.
	DialTimeout int `json:"dial_timeout,omitempty" example:"5"`

	// ReadTimeout is the MySQL read timeout in seconds. Appended as
	// `readTimeout=Ns` to the DSN. 0 leaves it unset.
	ReadTimeout int `json:"read_timeout,omitempty" example:"30"`

	// WriteTimeout is the MySQL write timeout in seconds. Appended as
	// `writeTimeout=Ns` to the DSN. 0 leaves it unset.
	WriteTimeout int `json:"write_timeout,omitempty" example:"10"`

	// SkipVersionCheck skips the default version probe performed by the
	// MySQL driver on first connect (mysql.Config.SkipInitializeWithVersion).
	// Encoded into the DSN as extra=skip_version_check:true and reapplied via
	// mysql.New(cfg) inside driver_mysql.go -- no driver.go modification needed.
	SkipVersionCheck bool `json:"skip_version_check,omitempty" example:"true"`

	// InterpolateParams enables client-side parameter interpolation in the
	// go-sql-driver. Appended as `interpolateParams=true` to the DSN.
	InterpolateParams bool `json:"interpolate_params,omitempty" example:"true"`

	// MultiStatements enables running multiple statements in a single query
	// (e.g. inside a migration script). Appended as `multiStatements=true`.
	// Security note: opt-in only; can allow SQL injection in multi-statement contexts.
	MultiStatements bool `json:"multi_statements,omitempty" example:"true"`

	// MaxAllowedPacket overrides the go-sql-driver default maximum packet size
	// (in bytes). Appended as `maxAllowedPacket=N` to the DSN. 0 leaves it unset.
	MaxAllowedPacket int `json:"max_allowed_packet,omitempty" example:"67108864"`

	// WithReturningDisabled requests the RETURNING clause to be disabled
	// for INSERT/UPDATE. For MySQL, applied via mysql.Config.DisableWithReturning
	// (an effective no-op semantically since MySQL doesn't use RETURNING by
	// default, but the flag is honored for parity). For Postgres, applied
	// via postgres.Config.WithoutReturning (PgBouncer compatibility).
	// Both are encoded as extra=without_returning:true in the DSN and
	// reapplied via <driver>.New(cfg) inside driver_<name>.go.
	WithReturningDisabled bool `json:"with_returning_disabled,omitempty" example:"true"`

	// SQLiteCache sets SQLite's cache mode. Values: "shared" (multiple
	// connections share a single in-memory cache), "private" (each connection
	// has its own cache). Empty string (the zero value) defaults to "shared"
	// to preserve historical behavior. Appended as the `cache=...` DSN parameter.
	SQLiteCache string `json:"sqlite_cache,omitempty" example:"private"`

	// SQLitePragmas applies PRAGMA settings to every new SQLite connection
	// via the modernc.org/sqlite inline pragma syntax (`_pragma=key(value)`
	// appended once per entry). Useful for tuning: `journal_mode=WAL`,
	// `busy_timeout=5000`, `foreign_keys=on`, `synchronous=NORMAL`, etc.
	SQLitePragmas map[string]string `json:"sqlite_pragmas,omitempty"`

	// ClickHouseTableEngine sets the default table engine for AutoMigrate
	// when creating tables without an explicit engine (clickhouse.Config.
	// DefaultTableEngineOpts). Encoded into the DSN as `extra=table_engine:...`
	// and reapplied via clickhouse.New(cfg) inside driver_clickhouse.go -- no
	// driver.go modification needed.
	ClickHouseTableEngine string `json:"clickhouse_table_engine,omitempty" example:"ENGINE=ReplicatedMergeTree('/clickhouse/tables/1/metrics','replica1') ORDER BY timestamp"`

	// ClickHouseCompression sets the default compression for new tables
	// during AutoMigrate (clickhouse.Config.DefaultCompression). Values:
	// "LZ4" (default, lossless), "ZSTD" (better ratio), "None" (no compression).
	// Encoded into the DSN as `extra=default_compression:<value>` and
	// reapplied via clickhouse.New(cfg).
	ClickHouseCompression string `json:"clickhouse_compression,omitempty" example:"ZSTD"`

	// ClickHouseGranularity sets the default index granularity (1 granule
	// = 8192 rows by default). Encoded into the DSN as
	// `extra=default_granularity:N` and reapplied via clickhouse.New(cfg).
	ClickHouseGranularity int `json:"clickhouse_granularity,omitempty" example:"3"`

	// SkipInitVersion skips the version probe performed by gorm.io/driver/clickhouse
	// (clickhouse.Config.SkipInitializeWithVersion). Encoded into the DSN as
	// `extra=skip_init_version:true` and reapplied via clickhouse.New(cfg).
	SkipInitVersion bool `json:"skip_init_version,omitempty" example:"true"`

	// ClickHouseDtPrecisionDisabled disables datetime precision at column
	// level for AutoMigrate (clickhouse.Config.DisableDatetimePrecision).
	// Encoded into the DSN as `extra=disable_datetime_precision:true` and
	// reapplied via clickhouse.New(cfg).
	ClickHouseDtPrecisionDisabled bool `json:"clickhouse_dt_precision_disabled,omitempty" example:"true"`

	// CreateBatchSize sets the default batch size for Create(). 0 means unlimited (GORM default).
	CreateBatchSize int `json:"create_batch_size,omitempty" example:"1000"`

	// DryRun generates SQL statements without executing them. Useful for debugging
	// or inspecting generated queries before running them against a real database.
	DryRun bool `json:"dry_run,omitempty" example:"true"`

	// SkipDefaultTransaction disables GORM's default transaction wrapping for
	// create/update/delete operations. Improves throughput when callers manage
	// their own transactions.
	SkipDefaultTransaction bool `json:"skip_default_transaction,omitempty" example:"true"`

	// TranslateError translates database driver errors into well-known GORM
	// errors (e.g., gorm.ErrDuplicatedKey) when possible.
	TranslateError bool `json:"translate_error,omitempty" example:"true"`

	// NowFunc overrides the function used by GORM to obtain the current time
	// (e.g., for "created_at" / "updated_at" defaults). Useful for deterministic tests.
	NowFunc func() time.Time `json:"-"`

	// AutoPingDisabled disables GORM's automatic ping on connection setup.
	AutoPingDisabled bool `json:"auto_ping_disabled,omitempty" example:"true"`

	// AllowGlobalUpdate permits UPDATE/DELETE without a WHERE clause
	// (e.g., `db.Model(&User{}).Update("name", "x")`). Use with care.
	AllowGlobalUpdate bool `json:"allow_global_update,omitempty" example:"true"`

	// QueryFields forces SELECT statements to use fully qualified column names
	// (e.g., `SELECT users.name FROM users` instead of `SELECT name FROM users`).
	QueryFields bool `json:"query_fields,omitempty" example:"true"`

	// KeepFKConstraints controls FK generation during AutoMigrate.
	// With KeepFKConstraints=true, FK constraints are generated during
	// migration. The mapping is inverted so that the zero value (false/default)
	// preserves the historical default (FK constraints are NOT generated).
	// Set KeepFKConstraints=true to opt-in to FK generation.
	KeepFKConstraints bool `json:"keep_fk_constraints,omitempty" example:"true"`

	// SingularTableDisabled disables singular table names (e.g., "users" instead of "user").
	// Default is false, meaning singular table names are enabled (current behavior).
	SingularTableDisabled bool `json:"singular_table_disabled,omitempty" example:"true"`

	// NamingStrategy sets a fully custom naming strategy. When non-nil,
	// SingularTableDisabled and TablePrefix are ignored.
	NamingStrategy schema.Namer `json:"-"`

	// TablePrefix is a prefix prepended to all table names.
	TablePrefix string `json:"table_prefix,omitempty" example:""`

	// AppName is the application name reported to the database server
	// (Postgres: application_name, SQL Server: app name).
	// When empty, falls back to the connection ConnName.
	AppName string `json:"app_name,omitempty" example:"order-service"`

	// Timezone sets the session timezone (e.g., "UTC", "Local").
	Timezone string `json:"timezone,omitempty" example:"UTC"`

	// SSLMode controls TLS/SSL. Values: "disable", "require",
	// "skip-verify", "preferred" (driver-dependent).
	SSLMode string `json:"ssl_mode,omitempty" example:"disable"`

	// PreferSimpleProtocol prefers the simple query protocol over the
	// extended one. Useful when connecting through PgBouncer in transaction
	// pooling mode (postgres.Config.PreferSimpleProtocol).
	// Encoded into the DSN as extra=prefer_simple_protocol:true and
	// reapplied via postgres.New(cfg) inside driver_postgres.go.
	PreferSimpleProtocol bool `json:"prefer_simple_protocol,omitempty" example:"true"`

	// ConnectTimeout is the Postgres connect timeout in seconds. Appended
	// as `connect_timeout=Ns` to the DSN. 0 leaves it unset (driver default).
	ConnectTimeout int `json:"connect_timeout,omitempty" example:"10"`

	// WithoutQuotingCheck disables identifier quoting checks performed by
	// the Postgres driver (postgres.Config.WithoutQuotingCheck).
	// Use with caution -- misquoted identifiers can corrupt queries.
	// Encoded into the DSN as extra=without_quoting_check:true and
	// reapplied via postgres.New(cfg) inside driver_postgres.go.
	WithoutQuotingCheck bool `json:"without_quoting_check,omitempty" example:"true"`

	// SSFailoverPartner is the database mirroring failover partner hostname.
	// Appended as `failoverPartner=host[:port]` to the MSSQL DSN. When
	// SSFailoverPort is set, the value combines into `host:port`.
	SSFailoverPartner string `json:"ss_failover_partner,omitempty" example:"secondary.example.com"`

	// SSFailoverPort is the failover partner port. Combined with
	// SSFailoverPartner to form the `failoverPartner` DSN parameter.
	SSFailoverPort string `json:"ss_failover_port,omitempty" example:"1433"`

	// SSWorkstation sets the workstation identifier reported to the server.
	// Appended as `workstation id=...` to the MSSQL DSN. Empty falls back to c.ConnName.
	SSWorkstation string `json:"ss_workstation,omitempty" example:"workstation-01"`

	// SSReadOnlyIntent routes the connection to a readable secondary.
	// Appended as `applicationintent=readonly` to the MSSQL DSN.
	SSReadOnlyIntent bool `json:"ss_read_only_intent,omitempty" example:"true"`

	// SSPacketSize is the TDS packet size in bytes (driver default 4096).
	// Appended as `packet size=N` to the MSSQL DSN. 0 leaves it unset.
	SSPacketSize int `json:"ss_packet_size,omitempty" example:"4096"`

	// SSRetryDisabled is RESERVED for future use. go-mssqldb 1.7.2 does NOT
	// expose any retry-related DSN parameter or sqlserver.Config field for
	// connection retry behavior, so setting this field has no current effect
	// on the open path. The field is included so callers can opt-in once the
	// upstream driver adds support; the BuildDSN/Open path is unchanged today.
	SSRetryDisabled bool `json:"ss_retry_disabled,omitempty" example:"true"`

	// SSDialTimeout is the MSSQL dial timeout in seconds. Appended as
	// `dial timeout=Ns` to the MSSQL DSN. 0 leaves it unset (driver default).
	SSDialTimeout int `json:"ss_dial_timeout,omitempty" example:"5"`

	// SSConnTimeout is the MSSQL login / connection timeout in seconds.
	// Appended as `connection timeout=Ns` to the MSSQL DSN. 0 leaves it unset.
	SSConnTimeout int `json:"ss_conn_timeout,omitempty" example:"30"`

	// SSKeepAlive is the MSSQL keepalive interval in seconds. Appended as
	// `keepalive=Ns` to the MSSQL DSN. 0 leaves it unset.
	SSKeepAlive int `json:"ss_keep_alive,omitempty" example:"30"`

	// DefaultStringSize sets the default string length used by AutoMigrate
	// for string columns that don't have an explicit size (sqlserver.Config.
	// DefaultStringSize and mysql.Config.DefaultStringSize). Encoded into the
	// DSN as `extra=default_string_size:N` and reapplied via <driver>.New(cfg)
	// inside driver_<name>.go -- no driver.go modification needed.
	DefaultStringSize int `json:"default_string_size,omitempty" example:"191"`

	// MaxOpenConns limits the maximum number of open connections.
	MaxOpenConns int `json:"max_open_conns,omitempty" example:"10"`

	// MaxIdleConns limits the maximum number of idle connections.
	MaxIdleConns int `json:"max_idle_conns,omitempty" example:"5"`

	// MaxIdleTime is the maximum idle time in seconds before a connection is closed.
	MaxIdleTime int `json:"max_idle_time,omitempty" example:"300"`

	// MaxLifeTime is the maximum lifetime in seconds for a connection.
	MaxLifeTime int `json:"max_life_time,omitempty" example:"3600"`

	// ExtraParams holds additional DSN query parameters not covered by other fields.
	ExtraParams string `json:"extra_params,omitempty" example:""`

	// AutoMigrate enables automatic schema migration on Connect().
	AutoMigrate bool `json:"auto_migrate,omitempty" example:"true"`

	// MigrationItems lists model structs to auto-migrate.
	MigrationItems []any `json:"-"`

	// MigrationSeeds lists seed data (raw models, Seed funcs, or Seeder interfaces).
	MigrationSeeds []any `json:"-"`
}

// FromDSN parses a DSN string and populates the Config struct
// It returns an error if the DSN is invalid
func (c *Config) FromDSN(dsn string) error {
	uri, err := url.Parse(dsn)
	if err == nil {
		uri.ForceQuery = true
		c.Type = uri.Scheme
		c.Host = uri.Hostname()
		if c.Host == "" {
			c.Host = "localhost"
		}

		c.Port = uri.Port()
		if c.Port == "" {
			switch uri.Scheme {
			case "mysql":
				c.Port = "3306"
			case "postgres", "postgresql":
				c.Port = "5432"
			case "cockroach":
				c.Port = "26257"
			case "tidb":
				c.Port = "4000"
			}
		}

		c.User = uri.User.Username()
		c.Pass, _ = uri.User.Password()
		c.Name = strings.Trim(uri.Path, "/")

		var val url.Values
		val, err = url.ParseQuery(uri.RawQuery)

		if err == nil {
			extraParams := url.Values{}
			for k := range val {
				switch k {
				case "name":
					c.ConnName = val.Get("name")

				case "charset":
					c.Charset = val.Get("charset")

				case "timezone":
					c.Timezone = val.Get("timezone")

				case "log_level":
					c.LogLevel = val.Get("log_level")

				case "log_not_found":
					c.LogNotFound = val.Get("log_not_found") == "true"

				case "table_prefix":
					c.TablePrefix = val.Get("table_prefix")

				case "max_open_conns":
					c.MaxOpenConns, _ = strconv.Atoi(val.Get("max_open_conns"))

				case "max_idle_conns":
					c.MaxIdleConns, _ = strconv.Atoi(val.Get("max_idle_conns"))

				case "max_idle_time":
					c.MaxIdleTime, _ = strconv.Atoi(val.Get("max_idle_time"))

				case "max_life_time":
					c.MaxLifeTime, _ = strconv.Atoi(val.Get("max_life_time"))

				case "auto_migrate":
					c.AutoMigrate = val.Get(k) == "true"

				case "sslmode":
					c.SSLMode = val.Get("sslmode")
					if c.SSLMode == "" {
						c.SSLMode = "disable"
					}

				default:
					fmt.Println("Setting up", k)
					extraParams.Set(k, val.Get(k))

				}
			}

			if len(extraParams) > 0 {
				c.ExtraParams = extraParams.Encode()
			}
		}

		if c.ConnName == "" {
			err = errors.New("dsn connection name is required")
		}
	}

	return err
}
