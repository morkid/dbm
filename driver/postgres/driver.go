package postgres

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/morkid/dbm"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func init() {
	dbm.RegisterDriver("postgres", dbm.ConnectionBuilder{
		BuildDSN: func(c dbm.Config) string {
			appName := c.AppName
			if appName == "" {
				appName = c.ConnName
			}
			appNameSlug := strings.ToLower(regexp.MustCompile(`[^A-z0-9-]`).ReplaceAllString(appName, "-"))

			dsnParams := url.Values{
				"host":             {c.Host},
				"port":             {c.Port},
				"user":             {c.User},
				"password":         {c.Pass},
				"dbname":           {c.Name},
				"sslmode":          {c.SSLMode},
				"application_name": {appNameSlug},
				"TimeZone":         {c.Timezone},
			}

			if c.ConnectTimeout > 0 {
				dsnParams.Set("connect_timeout", strconv.Itoa(c.ConnectTimeout)+"s")
			}

			if val, err := url.ParseQuery(c.ExtraParams); err == nil && len(val) > 0 {
				for k := range val {
					if !dsnParams.Has(k) {
						dsnParams.Set(k, val.Get(k))
					}
				}
			}

			params := []string{}
			for k := range dsnParams {
				unescaped := dsnParams.Get(k)
				u, e := url.QueryUnescape(dsnParams.Get(k))
				if e == nil {
					unescaped = u
				}

				params = append(params, fmt.Sprintf("%s=%s", k, unescaped))
			}

			// Encode dbm-specific Postgres config flags into the DSN as an
			// "extra" parameter with flag:value pairs separated by U+001F
			// (Unit Separator).  Unit Separator mirrors the Batch C
			// ClickHouse and Batch D MySQL convention: it is safe against
			// comma-in-value corruption (a future flag value containing a
			// literal comma would silently corrupt the parse if commas
			// were used as the separator).
			extraParams := []string{}
			if c.PreferSimpleProtocol {
				extraParams = append(extraParams, "prefer_simple_protocol:true")
			}
			if c.WithoutQuotingCheck {
				extraParams = append(extraParams, "without_quoting_check:true")
			}
			if c.WithReturningDisabled {
				extraParams = append(extraParams, "without_returning:true")
			}
			if len(extraParams) > 0 {
				params = append(params, "extra="+strings.Join(extraParams, "\x1F"))
			}

			return strings.Join(params, " ")
		},
		// Open receives the DSN produced by BuildDSN. If an "extra" key is
		// present, the embedded flags are applied to postgresDriver.Config struct
		// fields and the cleaned DSN (with "extra" stripped) is passed to
		// postgresDriver.New(cfg). If no extras are present, the zero-config
		// postgresDriver.Open fast path is preserved.
		Open: func(dsn string) gorm.Dialector {
			parts := strings.Split(dsn, " ")
			cleanParts := make([]string, 0, len(parts))
			extras := map[string]bool{}

			for _, p := range parts {
				if k, v, ok := strings.Cut(p, "="); ok && k == "extra" {
					for _, flag := range strings.Split(v, "\x1F") {
						if fk, fv, ok := strings.Cut(flag, ":"); ok && fv == "true" {
							extras[fk] = true
						}
					}
				} else {
					cleanParts = append(cleanParts, p)
				}
			}

			cleanDSN := strings.Join(cleanParts, " ")

			if len(extras) == 0 {
				return postgresDriver.Open(cleanDSN)
			}

			cfg := postgresDriver.Config{DSN: cleanDSN}
			if extras["prefer_simple_protocol"] {
				cfg.PreferSimpleProtocol = true
			}
			if extras["without_quoting_check"] {
				cfg.WithoutQuotingCheck = true
			}
			if extras["without_returning"] {
				cfg.WithoutReturning = true
			}
			return postgresDriver.New(cfg)
		},
		DefaultConfig: dbm.Config{
			ConnName:     "postgres",
			Host:         "localhost",
			Port:         "5432",
			User:         "postgres",
			Pass:         "postgres",
			Name:         "postgres",
			Timezone:     "UTC",
			MaxLifeTime:  3600,
			MaxIdleTime:  300,
			MaxIdleConns: 1,
			MaxOpenConns: 2,
		},
	})
}
