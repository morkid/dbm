# dbm

> Simple Database Connection Manager for GORM

[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8?logo=go)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/morkid/dbm.svg)](https://pkg.go.dev/github.com/morkid/dbm)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen)](.)
[![CI](https://github.com/morkid/dbm/actions/workflows/ci.yml/badge.svg)](https://github.com/morkid/dbm/actions/workflows/ci.yml)

**dbm** is a lightweight connection manager that wraps [GORM](https://gorm.io), giving you multi-database support, auto-migration, data seeding, and lifecycle hooks, all with a clean, minimal API.

---

## Features

- **Multi-Database** -- register and manage multiple named connections
- **Auto-Migrate** -- run schema migrations automatically on connect
- **Seeding** -- seed data using functions or interfaces
- **Hooks** -- lifecycle callbacks when connections are created
- **Connection Pooling** -- configure max open/idle connections, lifetime, etc.
- **DSN Parsing** -- parse connection URIs into config structs
- **Driver Registry** -- easily add new database drivers
- **Pluggable Drivers** -- MySQL, PostgreSQL, SQLite, SQL Server, ClickHouse, TiDB, CockroachDB available as opt-in subpackages
- **Tree-Shakeable Imports** -- import only the drivers you need (`driver/sqlite`, `driver/all`, ...)

## Supported Databases

| Driver      | dbm Subpackage                                                           | GORM Driver Package                                               |
| ----------- | ------------------------------------------------------------------------ | ----------------------------------------------------------------- |
| MySQL       | `github.com/morkid/dbm/driver/mysql`                                   | `gorm.io/driver/mysql`                                          |
| TiDB        | `github.com/morkid/dbm/driver/tidb`                                    | `gorm.io/driver/mysql` (MySQL-compatible wire protocol)         |
| Postgres    | `github.com/morkid/dbm/driver/postgres`                                | `gorm.io/driver/postgres`                                       |
| CockroachDB | `github.com/morkid/dbm/driver/cockroach`                               | `gorm.io/driver/postgres` (PostgreSQL-compatible wire protocol) |
| SQLite      | `github.com/morkid/dbm/driver/sqlite`                                  | `github.com/glebarez/sqlite` (pure Go, no CGO)                  |
| SQL Server  | `github.com/morkid/dbm/driver/mssql` (registers as Type `sqlserver`) | `gorm.io/driver/sqlserver`                                      |
| ClickHouse  | `github.com/morkid/dbm/driver/clickhouse`                              | `gorm.io/driver/clickhouse`                                     |

Drivers live in **per-driver subpackages** under `github.com/morkid/dbm/driver/<name>/` and auto-register themselves via `init()`. Use either:

- `import _ "github.com/morkid/dbm/driver/all"` -- pull in every supported driver
- `import _ "github.com/morkid/dbm/driver/<name>"` -- import only what you need (smaller binary)

Note: `dbm` is a deliberately **driver-agnostic core**. Without importing at least one driver subpackage, no `Config{Type: "..."}` value will resolve at `Connect()` time.

Adding a new driver is as simple as creating a `driver/<name>/` subpackage that calls `RegisterDriver` (see [Adding a Custom Driver](#adding-a-custom-driver)).

## Installation

```
go get github.com/morkid/dbm
```

Each database driver is a **separate opt-in subpackage** -- pick one of these:

```
# Import one specific driver
go get github.com/morkid/dbm/driver/sqlite

# ... or import every supported driver at once
go get github.com/morkid/dbm/driver/all
```

Available driver subpackages: `driver/mysql`, `driver/postgres`, `driver/sqlite`, `driver/mssql`, `driver/clickhouse`, `driver/tidb`, `driver/cockroach`, `driver/all`.

## Quick Start

```go
package main

import (
    "gorm.io/gorm"
    "github.com/morkid/dbm"

    // Pick the driver(s) you need. Use `driver/all` instead of
    // `driver/sqlite` here to enable every supported database.
    _ "github.com/morkid/dbm/driver/sqlite"
)

type User struct {
    gorm.Model
    Name string
}

func main() {
    mgr := dbm.New()

    // Register a connection with auto-migration
    mgr.Register("default", dbm.Config{
        AutoMigrate:    true,
        MigrationItems: []any{&User{}},
    })

    // Connect (migration runs automatically)
    mgr.Connect("default")

    // Get the connection
    db := mgr.GetDefault()
    db.Create(&User{Name: "Alice"})
}
```

## Usage

### Register & Connect

```go
mgr := dbm.New()

mgr.Register("users", dbm.Config{
    Type:           "postgres",
    Host:           "localhost",
    Port:           "5432",
    User:           "postgres",
    Pass:           "secret",
    Name:           "usersdb",
    AutoMigrate:    true,
    MigrationItems: []any{&User{}, &Account{}},
})

mgr.Register("cache", dbm.Config{
    Type: "sqlite",
    Name: "/tmp/cache.db",
})

// Connect on demand
mgr.Connect("users")
mgr.Connect("cache")

// Or connect immediately during registration
mgr.Register("logs", dbm.Config{Type: "sqlite"}, true)
```

### Get Connection

```go
// Get the default connection (panics if not set)
db := mgr.GetDefault()
db.Find(&users)

// Get a named connection (returns error if not found)
db, err := mgr.Get("users")
if err != nil {
    log.Fatal(err)
}
db.Where("active = ?", true).Find(&activeUsers)

// Set a different default
mgr.SetDefault("users")
```

### DSN Parsing

```go
conf := &dbm.Config{}
conf.FromDSN("mysql://user:pass@localhost:3306/mydb?charset=utf8mb4&parseTime=True&loc=Local")
```

### Auto-Migration & Seeding

```go
mgr.Register("app", dbm.Config{
    AutoMigrate:    true,
    MigrationItems: []any{&User{}, &Post{}},
    MigrationSeeds: []any{
        &User{Name: "admin"},                    // raw model
        dbm.Seed(func(tx *gorm.DB) error {       // seed function
            return tx.Create(&User{Name: "seed"}).Error
        }),
        &MySeeder{},                             // seeder interface
    },
})
```

### Lifecycle Hooks

```go
dbm.OnConnectionCreated(func(name string, db *gorm.DB) {
    log.Printf("connected: %s", name)
})
```

### Connection Pooling

```go
mgr.Register("db", dbm.Config{
    MaxOpenConns: 25,
    MaxIdleConns: 10,
    MaxIdleTime:  300, // seconds
    MaxLifeTime:  3600, // seconds
})
```

## Configuration

> "Supported Drivers" column:
>
> - `all` means the field is applied universally to every driver through `connection_manager.go` (gorm.Config / connection pool / migration hooks).
> - Driver-specific lists (e.g. `mysql, postgres`) name the driver subpackage (e.g. `driver/mysql/driver.go`) that consumes the field in its own `BuildDSN` / `Open`.
> - The driver name (`mysql`, `postgres`, `cockroach`, `tidb`, `sqlite`, `sqlserver`, `clickhouse`) is the `Config.Type` value -- it is the key the driver self-registers under inside `dbm`'s registry, not the subpackage directory name (`sqlserver` is registered by the `driver/mssql` subpackage).
>
> Fields not consumed by the active driver are silently ignored or, for SQL Server `SSLMode`, intentionally dropped to avoid producing invalid DSN.

| Field                             | Type                  | Default                 | Description                                                                                                                                                                                                                                                                                                                  | Supported Drivers                     |
| --------------------------------- | --------------------- | ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------- |
| `Type`                          | `string`            | `"sqlite"`            | Database driver type                                                                                                                                                                                                                                                                                                         | all                                   |
| `Host`                          | `string`            | driver-specific         | Host address                                                                                                                                                                                                                                                                                                                 | all                                   |
| `Port`                          | `string`            | driver-specific         | Port number                                                                                                                                                                                                                                                                                                                  | all                                   |
| `User`                          | `string`            | driver-specific         | Username                                                                                                                                                                                                                                                                                                                     | all                                   |
| `Pass`                          | `string`            | driver-specific         | Password                                                                                                                                                                                                                                                                                                                     | all                                   |
| `Name`                          | `string`            | `":memory:"` (sqlite) | Database name / path                                                                                                                                                                                                                                                                                                         | all                                   |
| `SSLMode`                       | `string`            | `""`                  | SSL mode (MySQL:`tls`; Postgres/CockroachDB: `sslmode`; MSSQL: also drives `encrypt` DSN -- `"disable"` -> `encrypt=disabled`, `"require"` -> `encrypt=true`, `"skip-verify"` -> `encrypt=true` + `trustservercertificate=true`, unknown values are ignored; value is lowercased and whitespace-trimmed) | mysql, postgres, cockroach, sqlserver |
| `PreferSimpleProtocol`          | `bool`              | `false`               | Prefer simple protocol (Postgres/CockroachDB:`extra=prefer_simple_protocol:true` decoded in Open, applied via `postgres.New(cfg)`)                                                                                                                                                                                       | postgres, cockroach                   |
| `ConnectTimeout`                | `int`               | `0`                   | Postgres/CockroachDB connect timeout (sec suffix)                                                                                                                                                                                                                                                                            | postgres, cockroach                   |
| `WithoutQuotingCheck`           | `bool`              | `false`               | Skip identifier quoting (Postgres/CockroachDB:`extra=without_quoting_check:true` decoded in Open, applied via `postgres.New(cfg)`)                                                                                                                                                                                       | postgres, cockroach                   |
| `SQLiteCache`                   | `string`            | `"shared"`            | SQLite cache mode (shared/private)                                                                                                                                                                                                                                                                                           | sqlite                                |
| `SQLitePragmas`                 | `map[string]string` | `nil`                 | SQLite PRAGMA settings, applied per connection                                                                                                                                                                                                                                                                               | sqlite                                |
| `Timezone`                      | `string`            | driver-specific         | Timezone                                                                                                                                                                                                                                                                                                                     | mysql, postgres, cockroach            |
| `TablePrefix`                   | `string`            | `""`                  | Table name prefix                                                                                                                                                                                                                                                                                                            | all                                   |
| `SingularTableDisabled`         | `bool`              | `false`               | Disable singular table names                                                                                                                                                                                                                                                                                                 | all                                   |
| `NamingStrategy`                | `schema.Namer`      | `nil`                 | Fully custom NamingStrategy (overrides TablePrefix + SingularTableDisabled)                                                                                                                                                                                                                                                  | all                                   |
| `ExtraParams`                   | `string`            | `""`                  | Extra query parameters (merged into DSN, driver-level set keys take precedence)                                                                                                                                                                                                                                              | all                                   |
| `Charset`                       | `string`            | `"utf8mb4"`           | Connection charset (MySQL/TiDB:`charset` DSN parameter; defaults to `utf8mb4` when empty)                                                                                                                                                                                                                                | mysql, tidb                           |
| `Collation`                     | `string`            | `""`                  | MySQL/TiDB connection collation                                                                                                                                                                                                                                                                                              | mysql, tidb                           |
| `DialTimeout`                   | `int`               | `0`                   | MySQL/TiDB connect timeout (sec suffix)                                                                                                                                                                                                                                                                                      | mysql, tidb                           |
| `ReadTimeout`                   | `int`               | `0`                   | MySQL/TiDB read timeout (sec suffix)                                                                                                                                                                                                                                                                                         | mysql, tidb                           |
| `WriteTimeout`                  | `int`               | `0`                   | MySQL/TiDB write timeout (sec suffix)                                                                                                                                                                                                                                                                                        | mysql, tidb                           |
| `SkipVersionCheck`              | `bool`              | `false`               | Skip version probe (MySQL/TiDB:`extra=skip_version_check:true` decoded in Open, applied via `mysql.New(cfg)`). Particularly useful for TiDB to avoid MySQL-specific version queries.                                                                                                                                     | mysql, tidb                           |
| `ClickHouseTableEngine`         | `string`            | `""`                  | Default table engine for AutoMigrate (clickhouse.Config.DefaultTableEngineOpts). Encoded as`extra=table_engine:VALUE` (U+001F-separated extras).                                                                                                                                                                           | clickhouse                            |
| `ClickHouseCompression`         | `string`            | `""`                  | Default compression for new tables (LZ4, ZSTD, None). Encoded as`extra=default_compression:VALUE`.                                                                                                                                                                                                                         | clickhouse                            |
| `ClickHouseGranularity`         | `int`               | `0` (driver default)  | Default index granularity. 0 = unset. Encoded as`extra=default_granularity:N`.                                                                                                                                                                                                                                             | clickhouse                            |
| `SkipInitVersion`               | `bool`              | `false`               | Skip version probe (clickhouse.Config.SkipInitializeWithVersion). Encoded as`extra=skip_init_version:true`.                                                                                                                                                                                                                | clickhouse                            |
| `ClickHouseDtPrecisionDisabled` | `bool`              | `false`               | Disable datetime precision for AutoMigrate. Encoded as`extra=disable_datetime_precision:true`.                                                                                                                                                                                                                             | clickhouse                            |
| `InterpolateParams`             | `bool`              | `false`               | go-sql-driver client-side interp                                                                                                                                                                                                                                                                                             | mysql, tidb                           |
| `MultiStatements`               | `bool`              | `false`               | Multi-statement queries (MySQL/TiDB)                                                                                                                                                                                                                                                                                         | mysql, tidb                           |
| `MaxAllowedPacket`              | `int`               | `0`                   | MySQL/TiDB packet override (bytes)                                                                                                                                                                                                                                                                                           | mysql, tidb                           |
| `WithReturningDisabled`         | `bool`              | `false`               | Disable RETURNING clause (Postgres/CockroachDB + MySQL/TiDB; encoded as`extra=without_returning:true` / `extra=disable_with_returning:true` and reapplied via `<driver>.New(cfg)`)                                                                                                                                     | mysql, tidb, postgres, cockroach      |
| `LogLevel`                      | `string`            | `"warn"`              | Log verbosity: silent, error, warn, info                                                                                                                                                                                                                                                                                     | all                                   |
| `Logger`                        | `logger.Interface`  | `nil`                 | Custom GORM logger (overrides LogLevel)                                                                                                                                                                                                                                                                                      | all                                   |
| `LogSlowThreshold`              | `int`               | `200`                 | Slow query threshold in ms                                                                                                                                                                                                                                                                                                   | all                                   |
| `LogColorful`                   | `bool`              | auto (stderr)           | Enable colored log output                                                                                                                                                                                                                                                                                                    | all                                   |
| `LogNotFound`                   | `bool`              | `false`               | Log record-not-found errors                                                                                                                                                                                                                                                                                                  | all                                   |
| `Plugins`                       | `[]gorm.Plugin`     | `nil`                 | GORM plugins registered on Connect()                                                                                                                                                                                                                                                                                         | all                                   |
| `PrepareStmt`                   | `bool`              | `false`               | Enable prepared statement caching                                                                                                                                                                                                                                                                                            | all                                   |
| `PrepareStmtMaxSize`            | `int`               | `0` (unlimited)       | Prepared statement cache size limit                                                                                                                                                                                                                                                                                          | all                                   |
| `PrepareStmtTTL`                | `int`               | `0` (no expiry)       | Prepared statement TTL (seconds, clamped to 0 if negative)                                                                                                                                                                                                                                                                   | all                                   |
| `CreateBatchSize`               | `int`               | `0` (unlimited)       | Default batch size for Create()                                                                                                                                                                                                                                                                                              | all                                   |
| `DryRun`                        | `bool`              | `false`               | Generate SQL without executing                                                                                                                                                                                                                                                                                               | all                                   |
| `SkipDefaultTransaction`        | `bool`              | `false`               | Disable GORM's default transaction                                                                                                                                                                                                                                                                                           | all                                   |
| `TranslateError`                | `bool`              | `false`               | Translate driver errors to GORM                                                                                                                                                                                                                                                                                              | all                                   |
| `NowFunc`                       | `func() time.Time`  | `nil`                 | Override "now" function for tests                                                                                                                                                                                                                                                                                            | all                                   |
| `AutoPingDisabled`              | `bool`              | `false`               | Disable GORM's automatic ping                                                                                                                                                                                                                                                                                                | all                                   |
| `AllowGlobalUpdate`             | `bool`              | `false`               | Allow UPDATE/DELETE without WHERE                                                                                                                                                                                                                                                                                            | all                                   |
| `QueryFields`                   | `bool`              | `false`               | Always prefix SELECT columns                                                                                                                                                                                                                                                                                                 | all                                   |
| `KeepFKConstraints`             | `bool`              | `false`               | Opt-in to FK generation during AutoMigrate                                                                                                                                                                                                                                                                                   | all                                   |
| `AppName`                       | `string`            | `""`                  | App name reported to server (Postgres/CockroachDB:`application_name`, MSSQL: `app name`). When empty, falls back to the connection ConnName.                                                                                                                                                                             | postgres, cockroach, sqlserver        |
| `SSFailoverPartner`             | `string`            | `""`                  | MSSQL database mirroring failover partner (DSN`failoverPartner=host[:port]`). Empty = no failover.                                                                                                                                                                                                                         | sqlserver                             |
| `SSFailoverPort`                | `string`            | `""`                  | Failover partner port; combined with SSFailoverPartner into`failoverPartner=host:port`.                                                                                                                                                                                                                                    | sqlserver                             |
| `SSWorkstation`                 | `string`            | `""`                  | Workstation identifier (DSN`workstation id=...`); empty falls back to `Config.ConnName`.                                                                                                                                                                                                                                 | sqlserver                             |
| `SSReadOnlyIntent`              | `bool`              | `false`               | Route connection to a readable secondary (DSN`applicationintent=readonly`).                                                                                                                                                                                                                                                | sqlserver                             |
| `SSPacketSize`                  | `int`               | `0` (driver default)  | TDS packet size in bytes (DSN`packet size=N`). 0 leaves driver default (4096).                                                                                                                                                                                                                                             | sqlserver                             |
| `SSRetryDisabled`               | `bool`              | `false`               | **RESERVED for future use.** go-mssqldb 1.7.2 has no retry-related DSN parameter or struct field, so this field has no current effect on the connection path. Included so callers can opt-in once the upstream driver adds support.                                                                                    | sqlserver                             |
| `SSDialTimeout`                 | `int`               | `0`                   | MSSQL dial timeout in seconds (DSN`dial timeout=Ns`).                                                                                                                                                                                                                                                                      | sqlserver                             |
| `SSConnTimeout`                 | `int`               | `0`                   | MSSQL login / connection timeout in seconds (DSN`connection timeout=Ns`).                                                                                                                                                                                                                                                  | sqlserver                             |
| `SSKeepAlive`                   | `int`               | `0`                   | MSSQL keepalive interval in seconds (DSN`keepalive=Ns`).                                                                                                                                                                                                                                                                   | sqlserver                             |
| `DefaultStringSize`             | `int`               | `0` (driver default)  | Default string length used by AutoMigrate for string columns without an explicit size. Encoded as`extra=default_string_size:N` in DSN, parsed in Open, applied via `<driver>.New(cfg)` for mysql/tidb (mysql.Config.DefaultStringSize) and sqlserver (sqlserver.Config.DefaultStringSize).                               | mysql, tidb, sqlserver                |
| `AutoMigrate`                   | `bool`              | `false`               | Run auto-migration on connect                                                                                                                                                                                                                                                                                                | all                                   |
| `MigrationItems`                | `[]any`             | `nil`                 | Models to migrate                                                                                                                                                                                                                                                                                                            | all                                   |
| `MigrationSeeds`                | `[]any`             | `nil`                 | Seed data (model / Seed / Seeder)                                                                                                                                                                                                                                                                                            | all                                   |
| `MaxOpenConns`                  | `int`               | `2`                   | Max open connections                                                                                                                                                                                                                                                                                                         | all                                   |
| `MaxIdleConns`                  | `int`               | `1`                   | Max idle connections                                                                                                                                                                                                                                                                                                         | all                                   |
| `MaxIdleTime`                   | `int`               | `300`                 | Max idle time (seconds)                                                                                                                                                                                                                                                                                                      | all                                   |
| `MaxLifeTime`                   | `int`               | `3600`                | Max connection lifetime (seconds)                                                                                                                                                                                                                                                                                            | all                                   |

## Adding a Custom Driver

If the engine you need isn't covered by a built-in `driver/<name>` subpackage (or you just want to point `dbm` at your own `gorm.Dialector`), you can plug it in from anywhere inside your application -- no PRs upstream, no fork. The recipe is: write a small package that calls `dbm.RegisterDriver` once in an `init()` function, then make sure your binary imports that package.

A custom driver is the right call when:

- The engine has its own `gorm.io/driver/<x>` and you just want a thin `dbm` wrapper around it.
- You're prototyping an in-memory or test backend (`mock`, `sqlmock`, a gRPC-backed fake, ...).
- You're shipping a private engine inside your own module and don't need to upstream it.

### The `ConnectionBuilder` contract

A `ConnectionBuilder` is the one struct you fill in. Three fields:

```go
type ConnectionBuilder struct {
    BuildDSN      func(config dbm.Config) string   // Config -> DSN string
    Open          func(dsn string) gorm.Dialector // DSN -> gorm.Dialector
    DefaultConfig dbm.Config                      // fallbacks for zero-valued Config
}
```

- **`BuildDSN`** receives a fully-resolved `dbm.Config` (driver defaults already applied) and returns the DSN string your GORM driver expects.
- **`Open`** receives that DSN and returns the `gorm.Dialector` that GORM dials with.
- **`DefaultConfig`** provides fallback values for unset `Config` fields -- typically `Host`, `Port`, `User`, `Pass`, `Name`, `Timezone`, and the connection-pool sizing fields.

### Step-by-step example

Below is a self-contained snippet that registers a brand-new `mycustom` dialect and immediately puts it to use. The function does two things, in order. First it calls `dbm.RegisterDriver("mycustom", dbm.ConnectionBuilder{...})`, which tells `dbm` how to turn a `dbm.Config` into a DSN string (`BuildDSN`) and which `gorm.Dialector` to open it with (`Open`). Then it creates a connection manager with `dbm.New()` and routes the new dialect into the manager via `mgr.Register("default", dbm.Config{Type: "mycustom", ...})`. Once `RegisterDriver` has run, `mgr.Connect("default")` and `mgr.Get("default")` flow exactly as they would for any built-in driver -- the dialect behaves like one from `dbm`'s own `driver/<name>` family, just declared inline.

```go
package mycustom

import (
    "fmt"
    "strings"
    "github.com/morkid/dbm"
    "github.com/glebarez/sqlite"
    "gorm.io/gorm"
)

func main() {
    dbm.RegisterDriver("mycustom", dbm.ConnectionBuilder{
        BuildDSN: func(c dbm.Config) string {
            return strings.Trim(c.Name + "?" + c.ExtraParams, "?")
        },
        Open: sqlite.Open,
        DefaultConfig: dbm.Config{
            ConnName:     "mycustom",
            Name:         ":memory:",
        },
    })

    mgr := dbm.New()
    mgr.Register("default", dbm.Config{
        Type: "mycustom",
        Name: "/data/foo.db",
    })
}
```

### Contributing a new built-in driver

To upstream a new driver into this repository, see [`CONTRIBUTING.md`](CONTRIBUTING.md).

## License

[MIT](LICENSE)
