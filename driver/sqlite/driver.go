package sqlite

import (
	"fmt"
	"net/url"

	sqliteDriver "github.com/glebarez/sqlite"
	"github.com/morkid/dbm"
)

func init() {
	dbm.RegisterDriver("sqlite", dbm.ConnectionBuilder{
		BuildDSN: func(c dbm.Config) string {
			params := url.Values{}

			// Preserve historical default of cache=shared when SQLiteCache is
			// left unset. Users opt-in to private mode by setting the field.
			cache := c.SQLiteCache
			if cache == "" {
				cache = "shared"
			}
			params.Set("cache", cache)

			// Each pragma becomes its own _pragma=key(value) pair in the query
			// string (modernc.org/sqlite inline pragma syntax). url.Values.Add
			// appends to the existing _pragma key so multiple pragmas coexist
			// as repeated key=value pairs.
			for k, v := range c.SQLitePragmas {
				params.Add("_pragma", fmt.Sprintf("%s(%s)", k, url.QueryEscape(v)))
			}

			// User-supplied extras flow through unless they would clobber
			// driver-level params already set above (cache, _pragma, file).
			if c.ExtraParams != "" {
				if extra, err := url.ParseQuery(c.ExtraParams); err == nil {
					for k := range extra {
						if !params.Has(k) {
							params.Set(k, extra.Get(k))
						}
					}
				}
			}

			return fmt.Sprintf("file:%s?%s", c.Name, params.Encode())
		},
		Open: sqliteDriver.Open,
		DefaultConfig: dbm.Config{
			ConnName:     "sqlite",
			Name:         ":memory:",
			MaxLifeTime:  3600,
			MaxIdleTime:  300,
			MaxIdleConns: 1,
			MaxOpenConns: 2,
		},
	})
}
