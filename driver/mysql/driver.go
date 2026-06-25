package mysql

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/morkid/dbm"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	dbm.RegisterDriver("mysql", dbm.ConnectionBuilder{
		BuildDSN: func(c dbm.Config) string {
			charset := c.Charset
			if charset == "" {
				charset = "utf8mb4"
			}

			params := url.Values{}
			params.Set("charset", charset)
			params.Set("parseTime", "True")
			if c.Timezone != "" {
				params.Set("loc", c.Timezone)
			}

			if c.SSLMode != "" && c.SSLMode != "disable" {
				switch c.SSLMode {
				case "skip-verify":
					params.Set("tls", "skip-verify")
				case "preferred":
					params.Set("tls", "preferred")
				default:
					params.Set("tls", "true")
				}
			}

			if c.Collation != "" {
				params.Set("collation", c.Collation)
			}
			if c.DialTimeout > 0 {
				params.Set("connect_timeout", strconv.Itoa(c.DialTimeout)+"s")
			}
			if c.ReadTimeout > 0 {
				params.Set("readTimeout", strconv.Itoa(c.ReadTimeout)+"s")
			}
			if c.WriteTimeout > 0 {
				params.Set("writeTimeout", strconv.Itoa(c.WriteTimeout)+"s")
			}
			if c.InterpolateParams {
				params.Set("interpolateParams", "true")
			}
			if c.MultiStatements {
				params.Set("multiStatements", "true")
			}
			if c.MaxAllowedPacket > 0 {
				params.Set("maxAllowedPacket", strconv.Itoa(c.MaxAllowedPacket))
			}

			if c.ExtraParams != "" {
				if extra, err := url.ParseQuery(c.ExtraParams); err == nil {
					for k := range extra {
						if !params.Has(k) {
							params.Set(k, extra.Get(k))
						}
					}
				}
			}

			// Encode dbm-specific MySQL config flags into the DSN query as
			// an "extra" parameter with key:value pairs separated by U+001F
			// (Unit Separator). Unit Separator mirrors the Batch C ClickHouse
			// choice: it is safe in URLs (percent-encoded to %1F by
			// url.Values.Encode, round-trips through url.ParseQuery), AND
			// it cannot appear in valid MySQL DDL/identifier values
			// (commas inside a future flag value would otherwise silently
			// corrupt the parse, matching the ClickHouse comma bug we fixed
			// in Batch C).
			//
			// The `extra` key is reserved by dbm for encoding these flags.
			// A user setting ExtraParams="extra=..." is silently dropped
			// (see ExtraParams loop above) so that our extras pipeline
			// stays the single source of truth.
			extraParams := []string{}
			if c.SkipVersionCheck {
				extraParams = append(extraParams, "skip_version_check:true")
			}
			if c.WithReturningDisabled {
				extraParams = append(extraParams, "disable_with_returning:true")
			}
			// DefaultStringSize: int -> uint conversion happens in Open via
			// strconv.Atoi + positive check. Encoded here regardless of the
			// underlying driver field type so that the Open path sees the
			// value uniformly.
			if c.DefaultStringSize > 0 {
				extraParams = append(extraParams, "default_string_size:"+strconv.Itoa(c.DefaultStringSize))
			}
			if len(extraParams) > 0 {
				params.Set("extra", strings.Join(extraParams, "\x1F"))
			}

			return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
				c.User,
				c.Pass,
				c.Host,
				c.Port,
				c.Name,
				params.Encode(),
			)
		},
		// Open receives the DSN produced by BuildDSN. If an "extra" key is
		// present in the URL query, the embedded flags are applied to
		// mysqlDriver.Config struct fields and the cleaned DSN (with "extra"
		// stripped) is passed to mysqlDriver.New(cfg). If no extras are present,
		// the zero-config mysqlDriver.Open fast path is preserved.
		Open: func(dsn string) gorm.Dialector {
			base := dsn
			queryStr := ""
			if i := strings.Index(dsn, "?"); i >= 0 {
				base = dsn[:i]
				queryStr = dsn[i+1:]
			}

			// url.ParseQuery on the (possibly empty) query substring is
			// sufficient for our DSN format; well-formed queries never
			// error so we ignore any error path.
			values, _ := url.ParseQuery(queryStr)
			all := values["extra"]
			delete(values, "extra")

			cleanedDSN := base
			if encoded := values.Encode(); encoded != "" {
				cleanedDSN = base + "?" + encoded
			}

			if len(all) == 0 || all[0] == "" {
				return mysqlDriver.Open(cleanedDSN)
			}

			cfg := mysqlDriver.Config{DSN: cleanedDSN}
			for _, flag := range strings.Split(all[0], "\x1F") {
				k, v, _ := strings.Cut(flag, ":")
				switch k {
				case "skip_version_check":
					cfg.SkipInitializeWithVersion = v == "true"
				case "disable_with_returning":
					cfg.DisableWithReturning = v == "true"
				case "default_string_size":
					// mysqlDriver.Config.DefaultStringSize is uint, but Config
					// stores it as int -- guard against negative / zero
					// by requiring n > 0 and clamping the conversion.
					if n, err := strconv.Atoi(v); err == nil && n > 0 {
						cfg.DefaultStringSize = uint(n)
					}
				}
			}
			return mysqlDriver.New(cfg)
		},
		DefaultConfig: dbm.Config{
			ConnName:     "mysql",
			Host:         "localhost",
			Port:         "3306",
			User:         "root",
			Pass:         "",
			Name:         "test",
			Timezone:     "Local",
			MaxLifeTime:  3600,
			MaxIdleTime:  300,
			MaxIdleConns: 1,
			MaxOpenConns: 2,
		},
	})
}
