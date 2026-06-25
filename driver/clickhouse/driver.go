package clickhouse

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/morkid/dbm"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

// parseClickhouseExtras decodes the value of a ClickHouse DSN `extra`
// parameter back into a clickhouse.Config struct. The extras string
// format is key:value pairs separated by U+001F (Unit Separator):
//
//	"table_engine:ENGINE=ReplicatedMergeTree('/path','replica')\x1Fdefault_compression:ZSTD\x1F..."
//
// Unit Separator is safe to use here because it cannot appear in valid
// ClickHouse DDL values (such as the comma-separated zoo_path arguments
// inside ReplicatedMergeTree). Unknown keys are silently ignored, and
// malformed values fall back to the clickhouse.Config zero value for
// that field.
//
// NOTE: AppName (application name) is NOT supported for ClickHouse
// through the dbm driver. Users who need the `appname=` DSN parameter
// (single-product client_name reported to ClickHouse) can pass it manually
// via ExtraParams (e.g. Config{ExtraParams: "appname=my-service/1.0.0"}).
func parseClickhouseExtras(first string) clickhouse.Config {
	cfg := clickhouse.Config{}
	if first == "" {
		return cfg
	}
	for _, flag := range strings.Split(first, "\x1F") {
		k, v, _ := strings.Cut(flag, ":")
		switch k {
		case "table_engine":
			cfg.DefaultTableEngineOpts = v
		case "default_compression":
			cfg.DefaultCompression = v
		case "default_granularity":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.DefaultGranularity = n
			}
		case "skip_init_version":
			cfg.SkipInitializeWithVersion = v == "true"
		case "disable_datetime_precision":
			cfg.DisableDatetimePrecision = v == "true"
		}
	}
	return cfg
}

func init() {
	dbm.RegisterDriver("clickhouse", dbm.ConnectionBuilder{
		BuildDSN: func(c dbm.Config) string {
			params := url.Values{}

			// Encode dbm-specific ClickHouse config flags as an "extra"
			// parameter with key:value pairs separated by U+001F (Unit
			// Separator). Unit Separator is safe in URLs (it is percent-
			// encoded to %1F by url.Values.Encode and round-trips
			// correctly through url.ParseQuery) AND cannot appear in
			// valid ClickHouse DDL values such as ReplicatedMergeTree
			// zoo_path arguments like '/p','r' that legitimately contain
			// commas -- switching from "," (previous) to "\x1F" makes
			// the round-trip safe for those values.
			extraParts := []string{}
			if c.ClickHouseTableEngine != "" {
				extraParts = append(extraParts, "table_engine:"+c.ClickHouseTableEngine)
			}
			if c.ClickHouseCompression != "" {
				extraParts = append(extraParts, "default_compression:"+c.ClickHouseCompression)
			}
			if c.ClickHouseGranularity > 0 {
				extraParts = append(extraParts, "default_granularity:"+strconv.Itoa(c.ClickHouseGranularity))
			}
			if c.SkipInitVersion {
				extraParts = append(extraParts, "skip_init_version:true")
			}
			if c.ClickHouseDtPrecisionDisabled {
				extraParts = append(extraParts, "disable_datetime_precision:true")
			}
			if len(extraParts) > 0 {
				params.Set("extra", strings.Join(extraParts, "\x1F"))
			}

			if c.ExtraParams != "" {
				if extra, err := url.ParseQuery(c.ExtraParams); err == nil {
					// The `extra` key is reserved by dbm for encoding
					// dbm-specific flags (table_engine, default_compression,
					// default_granularity, skip_init_version,
					// disable_datetime_precision). A user setting
					// ExtraParams="extra=..." is silently DROPPED here
					// so that our extras pipeline stays the single source
					// of truth. Set ExtraParams on a *different* key
					// (e.g. "compress=true") and it will pass through.
					for k := range extra {
						if !params.Has(k) {
							params.Set(k, extra.Get(k))
						}
					}
				}
			}

			dsn := fmt.Sprintf("clickhouse://%s:%s@%s:%s/%s",
				c.User,
				c.Pass,
				c.Host,
				c.Port,
				c.Name,
			)

			if encoded := params.Encode(); encoded != "" {
				dsn += "?" + encoded
			}

			return dsn
		},
		// Open receives the DSN produced by BuildDSN. If an "extra" key
		// is present, the decoded flags are applied via
		// parseClickhouseExtras + clickhouse.New(cfg). The "extra" key
		// is stripped from the DSN before passing to clickhouse.New.
		// If no extras are present (fast path) the original
		// clickhouse.Open DSN round-trips through unchanged.
		Open: func(dsn string) gorm.Dialector {
			base := dsn
			queryStr := ""
			if i := strings.Index(dsn, "?"); i >= 0 {
				base = dsn[:i]
				queryStr = dsn[i+1:]
			}

			values, _ := url.ParseQuery(queryStr)
			all := values["extra"]
			delete(values, "extra")

			cleanedDSN := base
			if encoded := values.Encode(); encoded != "" {
				cleanedDSN = base + "?" + encoded
			}

			if len(all) == 0 || all[0] == "" {
				return clickhouse.Open(cleanedDSN)
			}

			cfg := parseClickhouseExtras(all[0])
			cfg.DSN = cleanedDSN
			return clickhouse.New(cfg)
		},
		DefaultConfig: dbm.Config{
			ConnName:     "clickhouse",
			Host:         "localhost",
			Port:         "9000",
			User:         "default",
			Pass:         "",
			Name:         "default",
			Timezone:     "UTC",
			MaxLifeTime:  3600,
			MaxIdleTime:  300,
			MaxIdleConns: 1,
			MaxOpenConns: 2,
		},
	})
}
