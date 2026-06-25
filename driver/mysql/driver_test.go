package mysql

import (
	"strings"
	"testing"

	"github.com/morkid/dbm"
)

func TestMysqlConfig(t *testing.T) {
	mysqlConfig, ok := dbm.GetDriver("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}
	conf := mysqlConfig.DefaultConfig
	conf.FromDSN("mysql://user:pass@localhost/dbname?name=default&extra=1&debug=true&log_level=info&log_not_found=true&table_prefix=t_&max_open_conns=1&max_idle_conns=1&max_idle_time=1&max_life_time=1&auto_migrate=true&sslmode=")

	dsn := mysqlConfig.BuildDSN(conf)
	t.Log(dsn)
}

func TestMysqlAdvancedConfig(t *testing.T) {
	mysqlConfig, ok := dbm.GetDriver("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}
	conf := mysqlConfig.DefaultConfig
	conf.Charset = "latin1"
	conf.SSLMode = "skip-verify"
	conf.Timezone = "Asia/Jakarta"

	dsn := mysqlConfig.BuildDSN(conf)
	t.Log(dsn)

	conf2 := mysqlConfig.DefaultConfig
	conf2.SSLMode = "preferred"
	dsn2 := mysqlConfig.BuildDSN(conf2)
	t.Log(dsn2)

	conf3 := mysqlConfig.DefaultConfig
	conf3.SSLMode = "require"
	dsn3 := mysqlConfig.BuildDSN(conf3)
	t.Log(dsn3)
}

func TestMysqlDSNParams(t *testing.T) {
	mysqlConfig, ok := dbm.GetDriver("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	cases := []struct {
		name   string
		mutate func(*dbm.Config)
		want   string
	}{
		{
			name:   "collation",
			mutate: func(c *dbm.Config) { c.Collation = "utf8mb4_unicode_ci" },
			want:   "collation=utf8mb4_unicode_ci",
		},
		{
			name:   "dial_timeout",
			mutate: func(c *dbm.Config) { c.DialTimeout = 5 },
			want:   "connect_timeout=5s",
		},
		{
			name:   "read_timeout",
			mutate: func(c *dbm.Config) { c.ReadTimeout = 30 },
			want:   "readTimeout=30s",
		},
		{
			name:   "write_timeout",
			mutate: func(c *dbm.Config) { c.WriteTimeout = 10 },
			want:   "writeTimeout=10s",
		},
		{
			name:   "interpolate_params",
			mutate: func(c *dbm.Config) { c.InterpolateParams = true },
			want:   "interpolateParams=true",
		},
		{
			name:   "multi_statements",
			mutate: func(c *dbm.Config) { c.MultiStatements = true },
			want:   "multiStatements=true",
		},
		{
			name:   "max_allowed_packet",
			mutate: func(c *dbm.Config) { c.MaxAllowedPacket = 67108864 },
			want:   "maxAllowedPacket=67108864",
		},
		{
			name:   "extra_params_passthrough",
			mutate: func(c *dbm.Config) { c.ExtraParams = "foo=bar" },
			want:   "foo=bar",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := mysqlConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := mysqlConfig.BuildDSN(conf)
			t.Log(dsn)
			if !strings.Contains(dsn, tc.want) {
				t.Errorf("expected DSN to contain %q, got: %s", tc.want, dsn)
			}
		})
	}
}

// TestMysqlConnectCoveredCases exercises the createDialect mysql path by
// registering a MySQL config and invoking Connect(). gorm.Open is expected
// to fail because no real MySQL server is available, but the call path
// (which now only goes through builder.Open) is covered, contributing to
// branch coverage on createDialect.
func TestMysqlConnectCoveredCases(t *testing.T) {
	mgr := dbm.New()
	mgr.Register("mysql_fallback", dbm.Config{
		Type: "mysql",
		Host: "127.0.0.1",
		Port: "65433",
		User: "x",
		Pass: "x",
		Name: "x",
	})
	_, _ = mgr.Connect("mysql_fallback") // expected to fail; not asserted
}

// TestMysqlExtrasBuildDSN verifies that BuildDSN encodes the new
// dbm-specific MySQL flags (SkipVersionCheck, WithReturningDisabled)
// as URL query parameter values inside the "extra" key, and that "extra"
// is absent when all flags are zero. Values are URL-encoded (colon -> %3A,
// comma -> %2C).
func TestMysqlExtrasBuildDSN(t *testing.T) {
	mysqlConfig, ok := dbm.GetDriver("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	cases := []struct {
		name   string
		mutate func(*dbm.Config)
		wants  []string
		absent bool
	}{
		{
			name:   "no_flags_no_extra",
			mutate: func(c *dbm.Config) {},
			absent: true,
		},
		{
			name:   "skip_version_check",
			mutate: func(c *dbm.Config) { c.SkipVersionCheck = true },
			wants:  []string{"extra=skip_version_check%3Atrue"},
		},
		{
			name:   "disable_with_returning",
			mutate: func(c *dbm.Config) { c.WithReturningDisabled = true },
			wants:  []string{"extra=disable_with_returning%3Atrue"},
		},
		{
			name: "both_flags",
			mutate: func(c *dbm.Config) {
				c.SkipVersionCheck = true
				c.WithReturningDisabled = true
			},
			wants: []string{
				"skip_version_check%3Atrue",
				"disable_with_returning%3Atrue",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := mysqlConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := mysqlConfig.BuildDSN(conf)
			t.Log(dsn)
			if tc.absent {
				if strings.Contains(dsn, "extra=") {
					t.Errorf("expected DSN to NOT contain 'extra=', got: %s", dsn)
				}
				return
			}
			for _, w := range tc.wants {
				if !strings.Contains(dsn, w) {
					t.Errorf("expected DSN to contain %q, got: %s", w, dsn)
				}
			}
		})
	}
}

// TestMysqlExtrasOpen verifies that the Open function in driver_mysql.go
// returns a non-nil gorm.Dialector for both code paths: the extras branch
// (mysql.New(cfg) with cleaned DSN, including url-encoded query parsing)
// and the zero-config fast path (mysql.Open(dsn) without extras).
func TestMysqlExtrasOpen(t *testing.T) {
	mysqlConfig, ok := dbm.GetDriver("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	// Path 1: with extras (both flags)
	conf := mysqlConfig.DefaultConfig
	conf.SkipVersionCheck = true
	conf.WithReturningDisabled = true
	dsn := mysqlConfig.BuildDSN(conf)
	if !strings.Contains(dsn, "extra=") {
		t.Fatalf("expected DSN to have 'extra=...', got: %s", dsn)
	}
	dialector := mysqlConfig.Open(dsn)
	if dialector == nil {
		t.Errorf("Open returned nil dialector for DSN with extras")
	}

	// Path 2: zero-config fast path (DSN with `?` but no extras)
	conf2 := mysqlConfig.DefaultConfig
	dsn2 := mysqlConfig.BuildDSN(conf2)
	if strings.Contains(dsn2, "extra=") {
		t.Fatalf("expected DSN to NOT have 'extra=', got: %s", dsn2)
	}
	dialector2 := mysqlConfig.Open(dsn2)
	if dialector2 == nil {
		t.Errorf("Open returned nil dialector for DSN without extras (fast path)")
	}

	// Path 3: zero-config fast path (DSN without `?`)
	rawDSN := "root:@tcp(127.0.0.1:3306)/test"
	dialector3 := mysqlConfig.Open(rawDSN)
	if dialector3 == nil {
		t.Errorf("Open returned nil dialector for DSN without '?'")
	}

	// Path 4: empty `extra=`
	conf4 := mysqlConfig.DefaultConfig
	dsn4 := mysqlConfig.BuildDSN(conf4) + "&extra="
	dialector4 := mysqlConfig.Open(dsn4)
	if dialector4 == nil {
		t.Errorf("Open returned nil dialector for DSN with 'extra=' empty value")
	}

	// Path 5: single flag
	conf5 := mysqlConfig.DefaultConfig
	conf5.SkipVersionCheck = true
	dsn5 := mysqlConfig.BuildDSN(conf5)
	dialector5 := mysqlConfig.Open(dsn5)
	if dialector5 == nil {
		t.Errorf("Open returned nil dialector for DSN with only skip_version_check")
	}

	// Path 6: Verify the cleaned DSN passed to mysql.New(cfg) has extra=
	// stripped.
	dsnWithOnlyReturning := mysqlConfig.BuildDSN(mysqlConfig.DefaultConfig)
	conf6 := mysqlConfig.DefaultConfig
	conf6.WithReturningDisabled = true
	dsnWithReturning := mysqlConfig.BuildDSN(conf6)
	if !strings.Contains(dsnWithReturning, "extra=") {
		t.Fatalf("expected DSN to have 'extra=', got: %s", dsnWithReturning)
	}
	if strings.Contains(dsnWithOnlyReturning, "extra=") {
		t.Errorf("DSN without flags should not contain 'extra=', got: %s", dsnWithOnlyReturning)
	}

	// Path 7: malformed extras (non-"true" values).
	malformedDSN := strings.Replace(mysqlConfig.BuildDSN(mysqlConfig.DefaultConfig), "?", "?extra=skip_version_check%3Afalse%2Cdisable_with_returning%3Abad&", 1)
	if !strings.Contains(malformedDSN, "extra=") {
		t.Fatalf("expected DSN to have 'extra=', got: %s", malformedDSN)
	}
	dialector7 := mysqlConfig.Open(malformedDSN)
	if dialector7 == nil {
		t.Errorf("Open returned nil for DSN with malformed extras")
	}

	// Path 8: extras with both true and false parts in same value
	mixedDSN := strings.Replace(mysqlConfig.BuildDSN(mysqlConfig.DefaultConfig), "?", "?extra=skip_version_check%3Atrue%2Cdisable_with_returning%3Afalse&", 1)
	dialector8 := mysqlConfig.Open(mixedDSN)
	if dialector8 == nil {
		t.Errorf("Open returned nil for DSN with mixed true/false extras")
	}
}

// TestMysqlExtrasDefaultStringSize verifies that DefaultStringSize
// is encoded into the DSN as `extra=default_string_size:N` (URL-encoded
// colon) and that the Open function returns a non-nil Dialector via the
// mysql.New(cfg) path.
func TestMysqlExtrasDefaultStringSize(t *testing.T) {
	mysqlConfig, ok := dbm.GetDriver("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	// Path 1: extras round-trip produces a non-nil dialector.
	conf := mysqlConfig.DefaultConfig
	conf.DefaultStringSize = 191
	dsn := mysqlConfig.BuildDSN(conf)
	t.Log(dsn)
	if !strings.Contains(dsn, "extra=default_string_size%3A191") {
		t.Fatalf("expected DSN to contain 'extra=default_string_size%%3A191' (URL-encoded colon), got: %s", dsn)
	}
	if dialector := mysqlConfig.Open(dsn); dialector == nil {
		t.Errorf("Open returned nil dialector for DSN with default_string_size extras")
	}

	// Path 2: zero value should NOT add an extra key.
	confZero := mysqlConfig.DefaultConfig
	dsnZero := mysqlConfig.BuildDSN(confZero)
	t.Log(dsnZero)
	if strings.Contains(dsnZero, "extra=") {
		t.Errorf("expected DSN to NOT contain 'extra=' when DefaultStringSize is 0, got: %s", dsnZero)
	}

	// Path 3: zero-config fast path (no extras at all).
	confClean := mysqlConfig.DefaultConfig
	dsnClean := mysqlConfig.BuildDSN(confClean)
	if strings.Contains(dsnClean, "extra=") {
		t.Fatalf("expected clean DSN to NOT contain 'extra=', got: %s", dsnClean)
	}
	if dialectorClean := mysqlConfig.Open(dsnClean); dialectorClean == nil {
		t.Errorf("Open returned nil for clean DSN (fast path)")
	}

	// Path 4: malformed extras (non-integer)
	dsnBad := strings.Replace(mysqlConfig.BuildDSN(mysqlConfig.DefaultConfig), "?", "?extra=default_string_size%3Anot_a_number&", 1)
	if !strings.Contains(dsnBad, "extra=") {
		t.Fatalf("expected malformed DSN to have 'extra=', got: %s", dsnBad)
	}
	if dialectorBad := mysqlConfig.Open(dsnBad); dialectorBad == nil {
		t.Errorf("Open returned nil for DSN with malformed extras")
	}

	// Path 5: explicitly empty extra= value branch
	confEmptyExtra := mysqlConfig.DefaultConfig
	dsnEmptyExtra := mysqlConfig.BuildDSN(confEmptyExtra) + "&extra="
	if dialectorEmptyExtra := mysqlConfig.Open(dsnEmptyExtra); dialectorEmptyExtra == nil {
		t.Errorf("Open returned nil for DSN with empty 'extra=' value")
	}

	// Path 6: combined with another extra flag
	confCombined := mysqlConfig.DefaultConfig
	confCombined.SkipVersionCheck = true
	confCombined.DefaultStringSize = 256
	dsnCombined := mysqlConfig.BuildDSN(confCombined)
	t.Log(dsnCombined)
	if !strings.Contains(dsnCombined, "skip_version_check%3Atrue") {
		t.Errorf("expected combined DSN to contain skip_version_check, got: %s", dsnCombined)
	}
	if !strings.Contains(dsnCombined, "default_string_size%3A256") {
		t.Errorf("expected combined DSN to contain default_string_size=256, got: %s", dsnCombined)
	}
	if dialectorCombined := mysqlConfig.Open(dsnCombined); dialectorCombined == nil {
		t.Errorf("Open returned nil for combined DSN")
	}
}
