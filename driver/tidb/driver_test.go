package tidb

import (
	"strings"
	"testing"

	"github.com/morkid/dbm"
)

func TestTiDBConfig(t *testing.T) {
	tiConfig, ok := dbm.GetDriver("tidb")
	if !ok {
		t.Fatal("tidb driver not registered")
	}
	conf := tiConfig.DefaultConfig
	conf.FromDSN("tidb://user:pass@localhost/dbname?name=default&extra=1&debug=true&log_level=info&log_not_found=true&table_prefix=t_&max_open_conns=1&max_idle_conns=1&max_idle_time=1&max_life_time=1&auto_migrate=true&sslmode=")

	dsn := tiConfig.BuildDSN(conf)
	t.Log(dsn)
}

func TestTiDBAdvancedConfig(t *testing.T) {
	tiConfig, ok := dbm.GetDriver("tidb")
	if !ok {
		t.Fatal("tidb driver not registered")
	}
	conf := tiConfig.DefaultConfig
	conf.Charset = "latin1"
	conf.SSLMode = "skip-verify"
	conf.Timezone = "Asia/Jakarta"

	dsn := tiConfig.BuildDSN(conf)
	t.Log(dsn)

	conf2 := tiConfig.DefaultConfig
	conf2.SSLMode = "preferred"
	dsn2 := tiConfig.BuildDSN(conf2)
	t.Log(dsn2)

	conf3 := tiConfig.DefaultConfig
	conf3.SSLMode = "require"
	dsn3 := tiConfig.BuildDSN(conf3)
	t.Log(dsn3)
}

func TestTiDBDSNParams(t *testing.T) {
	tiConfig, ok := dbm.GetDriver("tidb")
	if !ok {
		t.Fatal("tidb driver not registered")
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
			conf := tiConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := tiConfig.BuildDSN(conf)
			t.Log(dsn)
			if !strings.Contains(dsn, tc.want) {
				t.Errorf("expected DSN to contain %q, got: %s", tc.want, dsn)
			}
		})
	}
}

func TestTiDBExtrasBuildDSN(t *testing.T) {
	tiConfig, ok := dbm.GetDriver("tidb")
	if !ok {
		t.Fatal("tidb driver not registered")
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
			name:   "default_string_size",
			mutate: func(c *dbm.Config) { c.DefaultStringSize = 191 },
			wants:  []string{"extra=default_string_size%3A191"},
		},
		{
			name: "all_three",
			mutate: func(c *dbm.Config) {
				c.SkipVersionCheck = true
				c.WithReturningDisabled = true
				c.DefaultStringSize = 256
			},
			wants: []string{
				"skip_version_check%3Atrue",
				"disable_with_returning%3Atrue",
				"default_string_size%3A256",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := tiConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := tiConfig.BuildDSN(conf)
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

func TestTiDBExtrasOpen(t *testing.T) {
	tiConfig, ok := dbm.GetDriver("tidb")
	if !ok {
		t.Fatal("tidb driver not registered")
	}

	// Path 1: with extras (both flags)
	conf := tiConfig.DefaultConfig
	conf.SkipVersionCheck = true
	conf.WithReturningDisabled = true
	dsn := tiConfig.BuildDSN(conf)
	if !strings.Contains(dsn, "extra=") {
		t.Fatalf("expected DSN to have 'extra=...', got: %s", dsn)
	}
	dialector := tiConfig.Open(dsn)
	if dialector == nil {
		t.Errorf("Open returned nil dialector for DSN with extras")
	}

	// Path 2: zero-config fast path (DSN with `?` but no extras)
	conf2 := tiConfig.DefaultConfig
	dsn2 := tiConfig.BuildDSN(conf2)
	if strings.Contains(dsn2, "extra=") {
		t.Fatalf("expected DSN to NOT have 'extra=', got: %s", dsn2)
	}
	dialector2 := tiConfig.Open(dsn2)
	if dialector2 == nil {
		t.Errorf("Open returned nil dialector for DSN without extras (fast path)")
	}

	// Path 3: zero-config fast path (DSN without `?`)
	rawDSN := "root:@tcp(127.0.0.1:4000)/test"
	dialector3 := tiConfig.Open(rawDSN)
	if dialector3 == nil {
		t.Errorf("Open returned nil dialector for DSN without '?'")
	}

	// Path 4: empty `extra=`
	conf4 := tiConfig.DefaultConfig
	dsn4 := tiConfig.BuildDSN(conf4) + "&extra="
	dialector4 := tiConfig.Open(dsn4)
	if dialector4 == nil {
		t.Errorf("Open returned nil dialector for DSN with 'extra=' empty value")
	}

	// Path 5: single flag
	conf5 := tiConfig.DefaultConfig
	conf5.SkipVersionCheck = true
	dsn5 := tiConfig.BuildDSN(conf5)
	dialector5 := tiConfig.Open(dsn5)
	if dialector5 == nil {
		t.Errorf("Open returned nil dialector for DSN with only skip_version_check")
	}

	// Path 6: default_string_size extras
	conf6 := tiConfig.DefaultConfig
	conf6.DefaultStringSize = 191
	dsn6 := tiConfig.BuildDSN(conf6)
	dialector6 := tiConfig.Open(dsn6)
	if dialector6 == nil {
		t.Errorf("Open returned nil dialector for DSN with default_string_size")
	}

	// Path 7: malformed extras (non-"true" values)
	malformedDSN := strings.Replace(tiConfig.BuildDSN(tiConfig.DefaultConfig), "?", "?extra=skip_version_check%3Afalse%2Cdisable_with_returning%3Abad&", 1)
	dialector7 := tiConfig.Open(malformedDSN)
	if dialector7 == nil {
		t.Errorf("Open returned nil for DSN with malformed extras")
	}

	// Path 8: extras with both true and false parts
	mixedDSN := strings.Replace(tiConfig.BuildDSN(tiConfig.DefaultConfig), "?", "?extra=skip_version_check%3Atrue%2Cdisable_with_returning%3Afalse&", 1)
	dialector8 := tiConfig.Open(mixedDSN)
	if dialector8 == nil {
		t.Errorf("Open returned nil for DSN with mixed true/false extras")
	}
}

func TestTiDBConnectCoveredCases(t *testing.T) {
	mgr := dbm.New()
	mgr.Register("tidb_fallback", dbm.Config{
		Type: "tidb",
		Host: "127.0.0.1",
		Port: "4000",
		User: "root",
		Pass: "",
		Name: "test",
	})
	_, _ = mgr.Connect("tidb_fallback") // expected to fail; not asserted
}
