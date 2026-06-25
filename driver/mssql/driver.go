package mssql

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/morkid/dbm"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

func init() {
	dbm.RegisterDriver("sqlserver", dbm.ConnectionBuilder{
		BuildDSN: func(c dbm.Config) string {
			params := url.Values{}
			params.Set("database", c.Name)

			appName := c.AppName
			if appName == "" {
				appName = c.ConnName
			}
			params.Set("app name", appName)

			// Reuse existing SSLMode to drive go-mssqldb encrypt / trust
			// DSN params. Valid go-mssqldb values for `encrypt` are: "true",
			// "false", "strict", "disable". "required" is rejected by the
			// driver. Mapping follows the semantic intent of each SSLMode
			// value rather than a literal pass-through:
			//   ""              -> no encrypt key (go-mssqldb default = optional)
			//   "disable"       -> encrypt=disabled   (no TLS)
			//   "require"       -> encrypt=true        (TLS required, cert validated)
			//   "skip-verify"   -> encrypt=true +
			//                      trustservercertificate=true
			//                      (TLS required, but skip cert verification;
			//                      go-mssqldb has no single DSN value for this
			//                      combination, so both params must be set)
			//   other           -> unknown: no-op, leave it unset (go-mssqldb
			//                      defaults to optional). Forwarding unknown
			//                      values as `encrypt=<unknown>` would silently
			//                      produce invalid DSN params that go-mssqldb
			//                      mistreats, so we drop unknown values.
			//
			// Precedence: SSLMode is the canonical config. ExtraParams is
			// only merged for keys that SSLMode did not set, so SSLMode
			// wins for `encrypt` / `trustservercertificate`. To override via
			// ExtraParams, leave SSLMode empty.
			sslMode := strings.ToLower(strings.TrimSpace(c.SSLMode))
			switch sslMode {
			case "require":
				params.Set("encrypt", "true")
			case "skip-verify":
				params.Set("encrypt", "true")
				params.Set("trustservercertificate", "true")
			case "disable":
				params.Set("encrypt", "disabled")
			case "":
				// leave it unset; go-mssqldb defaults to optional encryption negotiation
			default:
				// unknown value intentionally not forwarded; see comment above
			}

			// Route the connection to a readable secondary (read-only routing).
			if c.SSReadOnlyIntent {
				params.Set("applicationintent", "readonly")
			}

			// TDS packet size. Driver default is 4096; non-zero overrides.
			if c.SSPacketSize > 0 {
				params.Set("packet size", strconv.Itoa(c.SSPacketSize))
			}

			// Workstation identifier. Empty falls back to c.Name so a sensible
			// identifier is always reported.
			workstation := c.SSWorkstation
			if workstation == "" {
				workstation = c.ConnName
			}
			if workstation != "" {
				params.Set("workstation id", workstation)
			}

			// Database mirroring failover partner. Combine host[:port].
			if c.SSFailoverPartner != "" {
				partner := c.SSFailoverPartner
				if c.SSFailoverPort != "" {
					partner = partner + ":" + c.SSFailoverPort
				}
				params.Set("failoverPartner", partner)
			}

			// Timeouts in seconds, formatted as Go duration strings
			// (`Ns` = N seconds). go-mssqldb's parseDuration accepts both
			// `5s` / `1m` / etc., so `Ns` is the simplest unambiguous form.
			if c.SSDialTimeout > 0 {
				params.Set("dial timeout", strconv.Itoa(c.SSDialTimeout)+"s")
			}
			if c.SSConnTimeout > 0 {
				params.Set("connection timeout", strconv.Itoa(c.SSConnTimeout)+"s")
			}
			if c.SSKeepAlive > 0 {
				params.Set("keepalive", strconv.Itoa(c.SSKeepAlive)+"s")
			}

			// SSRetryDisabled: reserved for future use. go-mssqldb has no
			// DSN param / struct field today, so we don't set anything.

			// Extras pattern for struct-only fields. go-mssqldb does not
			// expose these via DSN, so we encode them as `extra=key:value`
			// pairs separated by U+001F (Unit Separator) and parse them
			// back below in Open.
			extraParts := []string{}
			if c.DefaultStringSize > 0 {
				extraParts = append(extraParts, "default_string_size:"+strconv.Itoa(c.DefaultStringSize))
			}
			if len(extraParts) > 0 {
				params.Set("extra", strings.Join(extraParts, "\x1F"))
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

			return fmt.Sprintf("sqlserver://%s:%s@%s:%s?%s",
				c.User,
				c.Pass,
				c.Host,
				c.Port,
				params.Encode(),
			)
		},
		// Open receives the DSN produced by BuildDSN. If an "extra" key is
		// present in the URL query, the embedded flags are applied to
		// sqlserver.Config struct fields and the cleaned DSN (with "extra"
		// stripped) is passed to sqlserver.New(cfg). If no extras are
		// present, the zero-config sqlserver.Open fast path is preserved.
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
				return sqlserver.Open(cleanedDSN)
			}

			cfg := sqlserver.Config{DSN: cleanedDSN}
			for _, flag := range strings.Split(all[0], "\x1F") {
				k, v, _ := strings.Cut(flag, ":")
				switch k {
				case "default_string_size":
					if n, err := strconv.Atoi(v); err == nil && n > 0 {
						cfg.DefaultStringSize = n
					}
				}
			}
			return sqlserver.New(cfg)
		},
		DefaultConfig: dbm.Config{
			ConnName:     "sqlserver",
			Host:         "localhost",
			Port:         "1433",
			User:         "sa",
			Pass:         "",
			Name:         "master",
			Timezone:     "UTC",
			MaxLifeTime:  3600,
			MaxIdleTime:  300,
			MaxIdleConns: 1,
			MaxOpenConns: 2,
		},
	})
}
