package cockroach

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/morkid/dbm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func init() {
	dbm.RegisterDriver("cockroach", dbm.ConnectionBuilder{
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
				return postgres.Open(cleanDSN)
			}

			cfg := postgres.Config{DSN: cleanDSN}
			if extras["prefer_simple_protocol"] {
				cfg.PreferSimpleProtocol = true
			}
			if extras["without_quoting_check"] {
				cfg.WithoutQuotingCheck = true
			}
			if extras["without_returning"] {
				cfg.WithoutReturning = true
			}
			return postgres.New(cfg)
		},
		DefaultConfig: dbm.Config{
			ConnName:     "cockroach",
			Host:         "localhost",
			Port:         "26257",
			User:         "root",
			Pass:         "root",
			Name:         "defaultdb",
			Timezone:     "UTC",
			MaxLifeTime:  3600,
			MaxIdleTime:  300,
			MaxIdleConns: 1,
			MaxOpenConns: 2,
		},
	})
}
