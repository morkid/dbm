package tidb

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/morkid/dbm"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	dbm.RegisterDriver("tidb", dbm.ConnectionBuilder{
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

			extraParams := []string{}
			if c.SkipVersionCheck {
				extraParams = append(extraParams, "skip_version_check:true")
			}
			if c.WithReturningDisabled {
				extraParams = append(extraParams, "disable_with_returning:true")
			}
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
				return mysql.Open(cleanedDSN)
			}

			cfg := mysql.Config{DSN: cleanedDSN}
			for _, flag := range strings.Split(all[0], "\x1F") {
				k, v, _ := strings.Cut(flag, ":")
				switch k {
				case "skip_version_check":
					cfg.SkipInitializeWithVersion = v == "true"
				case "disable_with_returning":
					cfg.DisableWithReturning = v == "true"
				case "default_string_size":
					if n, err := strconv.Atoi(v); err == nil && n > 0 {
						cfg.DefaultStringSize = uint(n)
					}
				}
			}
			return mysql.New(cfg)
		},
		DefaultConfig: dbm.Config{
			ConnName:     "tidb",
			Host:         "localhost",
			Port:         "4000",
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
