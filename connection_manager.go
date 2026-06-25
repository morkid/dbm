package dbm

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// sqliteDuplicateIndexPattern matches the error message sqlite emits
// when a duplicate index is created during AutoMigrate. Compiled once
// at package init time and reused across Migrate invocations to avoid
// re-compiling the regex per error.
var sqliteDuplicateIndexPattern = regexp.MustCompile("index ([^ ]+) already exists")

var _ Connection = &connectionManager{}

type connectionManager struct {
	dbs         map[string]*gorm.DB
	configs     map[string]Config
	defaultName string
}

func (c *connectionManager) Register(name string, config Config, connect ...bool) error {
	if c.dbs == nil {
		c.dbs = map[string]*gorm.DB{}
		c.configs = map[string]Config{}
	}

	var err error

	if _, ok := c.dbs[name]; !ok {
		if len(c.configs) == 0 {
			c.SetDefault(name)
		}

		c.configs[name] = config

		c.dbs[name] = nil

		if len(connect) > 0 && connect[0] {
			_, err = c.Connect(name)
		}
	}

	return err
}

func (c *connectionManager) Connect(name string, override ...bool) (dbConn *gorm.DB, err error) {
	var config *Config
	config, err = c.defaultConfigFor(name)

	if err == nil {
		logLevel := logger.Warn
		if config.LogLevel != "" {
			switch strings.ToLower(config.LogLevel) {
			case "silent":
				logLevel = logger.Silent
			case "error":
				logLevel = logger.Error
			case "warn":
				logLevel = logger.Warn
			case "info":
				logLevel = logger.Info
			}
		}

		slowThreshold := 200 * time.Millisecond
		if config.LogSlowThreshold > 0 {
			slowThreshold = time.Duration(config.LogSlowThreshold) * time.Millisecond
		}

		colorful := config.LogColorful
		if !colorful {
			if logFile, ok := log.Writer().(*os.File); ok && logFile == os.Stderr {
				colorful = true
			}
		}

		var gormLogger logger.Interface
		if config.Logger != nil {
			gormLogger = config.Logger
		} else {
			gormLogger = logger.New(log.New(log.Writer(), "[GORM] ", log.LstdFlags), logger.Config{
				SlowThreshold:             slowThreshold,
				LogLevel:                  logLevel,
				IgnoreRecordNotFoundError: !config.LogNotFound,
				Colorful:                  colorful,
			})
		}

		namingStrategy := config.NamingStrategy
		if namingStrategy == nil {
			namingStrategy = schema.NamingStrategy{
				TablePrefix:   config.TablePrefix,
				SingularTable: !config.SingularTableDisabled,
			}
		}

		// gorm.Config field names: PrepareStmtMaxSize (int) and PrepareStmtTTL
		// (time.Duration). PrepareStmtTTL is in seconds (int) and is converted.
		// Negative TTL is clamped to 0 to avoid zero/negative Duration semantics
		// that would be interpreted as instant cache expiry.
		prepareStmtTTL := config.PrepareStmtTTL
		if prepareStmtTTL < 0 {
			prepareStmtTTL = 0
		}
		cnf := gorm.Config{
			Logger:                                   gormLogger,
			NamingStrategy:                           namingStrategy,
			DisableForeignKeyConstraintWhenMigrating: !config.KeepFKConstraints,
			PrepareStmt:                              config.PrepareStmt,
			PrepareStmtMaxSize:                       config.PrepareStmtMaxSize,
			PrepareStmtTTL:                           time.Duration(prepareStmtTTL) * time.Second,
			CreateBatchSize:                          config.CreateBatchSize,
			DryRun:                                   config.DryRun,
			SkipDefaultTransaction:                   config.SkipDefaultTransaction,
			TranslateError:                           config.TranslateError,
			NowFunc:                                  config.NowFunc,
			DisableAutomaticPing:                     config.AutoPingDisabled,
			AllowGlobalUpdate:                        config.AllowGlobalUpdate,
			QueryFields:                              config.QueryFields,
		}

		var dialect gorm.Dialector
		dialect, err = c.createDialect(name)

		if err == nil {
			dbConn, err = gorm.Open(dialect, &cnf)
			if dbConn != nil && err == nil {
				isNew := len(override) == 0 || override[0]
				if isNew {
					c.dbs[name] = dbConn
				}

				if len(onConnectionCreates) > 0 {
					for i := range onConnectionCreates {
						if onConnectionCreates[i] != nil {
							onConnectionCreates[i](name, dbConn)
						}
					}
				}

				for i := range config.Plugins {
					if config.Plugins[i] != nil {
						if err = dbConn.Use(config.Plugins[i]); err != nil {
							break
						}
					}
				}

				var sqlDB *sql.DB
				if sqlDB, err = dbConn.DB(); err == nil {
					sqlDB.SetMaxOpenConns(config.MaxOpenConns)
					sqlDB.SetMaxIdleConns(config.MaxIdleConns)
					sqlDB.SetConnMaxIdleTime(time.Second * time.Duration(config.MaxIdleTime))
					sqlDB.SetConnMaxLifetime(time.Second * time.Duration(config.MaxLifeTime))

					if isNew && c.configs[name].AutoMigrate && len(c.configs[name].MigrationItems) > 0 {
						err = c.Migrate(name)
					}
				}
			}
		}
	}

	return dbConn, err
}

func (c *connectionManager) Migrate(name string) (err error) {
	var dbConn *gorm.DB
	dbConn, err = c.Connect(name, false)

	if dbConn != nil && err == nil {
		var sqlDB *sql.DB
		if sqlDB, err = dbConn.DB(); err == nil {
			defer sqlDB.Close()

			var config *Config
			config, err = c.defaultConfigFor(name)

			if err == nil {
				for i := range config.MigrationItems {
					err = dbConn.AutoMigrate(config.MigrationItems[i])
					sqliteDuplicateIndex := false

					if err != nil {
						// ignore sqlite duplicate indexes
						sqliteDuplicateIndex = sqliteDuplicateIndexPattern.MatchString(err.Error())
						if !sqliteDuplicateIndex {
							break
						}
					}

					if sqliteDuplicateIndex {
						err = nil
					}
				}

				if err != nil {
					fmt.Println("migrate error:", err.Error())
				}

				if err == nil {
					if len(config.MigrationSeeds) > 0 {
						for i := range config.MigrationSeeds {
							dbConn.Transaction(func(tx *gorm.DB) error {
								if seed, ok := (config.MigrationSeeds[i]).(Seed); ok {
									return seed(tx)
								} else if seed, ok := (config.MigrationSeeds[i]).(Seeder); ok {
									return seed.Seed(tx)
								}

								return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(config.MigrationSeeds[i]).Error
							})
						}
					}
				}
			}

		}

	}

	return err
}

func (c *connectionManager) createDialect(name string) (gorm.Dialector, error) {
	config, err := c.defaultConfigFor(name)
	if err != nil {
		return nil, err
	}

	builder, ok := drivers[config.Type]
	if !ok {
		return nil, fmt.Errorf("%s driver is not available", config.Type)
	}

	dsn := builder.BuildDSN(*config)
	return builder.Open(dsn), nil
}

func (c *connectionManager) defaultConfigFor(name string) (*Config, error) {
	var config *Config
	conf, ok := c.configs[name]
	if !ok {
		return nil, fmt.Errorf("no default configuration for %s", name)
	}

	config = &conf

	if config.Type == "" {
		config.Type = "sqlite"
	}

	defConf := Config{}

	if driver, ok := drivers[config.Type]; ok {
		defConf = driver.DefaultConfig
	}

	if config.ConnName == "" {
		config.ConnName = name
	}

	if config.Host == "" {
		config.Host = defConf.Host
	}

	if config.User == "" {
		config.User = defConf.User
	}

	if config.Pass == "" {
		config.Pass = defConf.Pass
	}

	if config.Port == "" {
		config.Port = defConf.Port
	}

	if config.Timezone == "" {
		config.Timezone = defConf.Timezone
	}

	if config.Name == "" && config.Type == "sqlite" {
		config.Name = ":memory:"
	}

	if config.MaxLifeTime == 0 {
		config.MaxLifeTime = 3600
		if defConf.MaxLifeTime > 0 {
			config.MaxLifeTime = defConf.MaxLifeTime
		}
	}

	if config.MaxIdleTime == 0 {
		config.MaxIdleTime = 300
		if defConf.MaxIdleTime > 0 {
			config.MaxIdleTime = defConf.MaxIdleTime
		}
	}

	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 1
		if defConf.MaxIdleConns > 0 {
			config.MaxIdleConns = defConf.MaxIdleConns
		}
	}

	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 2
		if defConf.MaxOpenConns > 0 {
			config.MaxOpenConns = defConf.MaxOpenConns
		}
	}

	return config, nil
}

func (c *connectionManager) Get(name string) (*gorm.DB, error) {
	dbConn, ok := c.dbs[name]

	if !ok {
		return nil, fmt.Errorf("no database connection for %s", name)
	}

	return dbConn, nil
}

func (c *connectionManager) SetDefault(name string) {
	c.defaultName = name
}

func (c *connectionManager) GetDefault() *gorm.DB {
	if c.defaultName == "" {
		c.defaultName = "default"
	}

	dbConn, err := c.Get(c.defaultName)
	if err != nil {
		panic(err.Error())
	}

	return dbConn
}

// New creates a new connection manager
// It returns a pointer to a connectionManager struct
func New() Connection {
	return new(connectionManager)
}
