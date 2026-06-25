package postgres

import (
	"net/url"
	"strings"
	"testing"

	"github.com/morkid/dbm"
)

func TestPostgresConfig(t *testing.T) {
	pgConfig, ok := dbm.GetDriver("postgres")
	if !ok {
		t.Fatal("postgres driver not registered")
	}
	conf := pgConfig.DefaultConfig
	conf.FromDSN("postgres://user:pass@localhost/dbname?sslmode=disable&extra=1")
	conf.Timezone = url.QueryEscape("Asia/Jakarta")

	dsn := pgConfig.BuildDSN(conf)
	t.Log(dsn)
}

func TestPostgresAppName(t *testing.T) {
	pgConfig, ok := dbm.GetDriver("postgres")
	if !ok {
		t.Fatal("postgres driver not registered")
	}

	// With explicit AppName.
	conf := pgConfig.DefaultConfig
	conf.AppName = "OrderService"
	conf.ConnName = "ignored"
	dsn := pgConfig.BuildDSN(conf)
	t.Log(dsn)
	if !strings.Contains(dsn, "application_name=orderservice") {
		t.Errorf("expected application_name=orderservice, got: %s", dsn)
	}

	// Without AppName: fallback to conf.ConnName.
	confFallback := pgConfig.DefaultConfig
	confFallback.AppName = ""
	confFallback.ConnName = "fallback-name"
	dsnFallback := pgConfig.BuildDSN(confFallback)
	t.Log(dsnFallback)
	if !strings.Contains(dsnFallback, "application_name=fallback-name") {
		t.Errorf("expected fallback application_name=fallback-name, got: %s", dsnFallback)
	}
}

// TestPostgresDSNParams verifies that driver_postgres.go BuildDSN appends
// the new ConnectTimeout DSN parameter when set, and that ExtraParams
// continues to flow through correctly.
func TestPostgresDSNParams(t *testing.T) {
	pgConfig, ok := dbm.GetDriver("postgres")
	if !ok {
		t.Fatal("postgres driver not registered")
	}

	cases := []struct {
		name   string
		mutate func(*dbm.Config)
		want   string
	}{
		{
			name:   "connect_timeout",
			mutate: func(c *dbm.Config) { c.ConnectTimeout = 10 },
			want:   "connect_timeout=10s",
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
			conf := pgConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := pgConfig.BuildDSN(conf)
			t.Log(dsn)
			if !strings.Contains(dsn, tc.want) {
				t.Errorf("expected DSN to contain %q, got: %s", tc.want, dsn)
			}
		})
	}
}

// TestPostgresConnectCoveredCases exercises the createDialect postgres path
// by registering a Postgres config and invoking Connect(). gorm.Open is
// expected to fail because no real Postgres server is available, but the
// call path (which now only goes through builder.Open) is covered.
func TestPostgresConnectCoveredCases(t *testing.T) {
	mgr := dbm.New()
	mgr.Register("pg_fallback", dbm.Config{
		Type: "postgres",
		Host: "127.0.0.1",
		Port: "65435",
		User: "x",
		Pass: "x",
		Name: "x",
	})
	_, _ = mgr.Connect("pg_fallback") // expected to fail; not asserted
}

// TestPostgresExtrasBuildDSN verifies that BuildDSN encodes the new
// dbm-specific Postgres flags (PreferSimpleProtocol, WithoutQuotingCheck,
// WithReturningDisabled) into an "extra" keyword in the space-separated
// DSN, and that the "extra" keyword is absent when all flags are zero.
func TestPostgresExtrasBuildDSN(t *testing.T) {
	pgConfig, ok := dbm.GetDriver("postgres")
	if !ok {
		t.Fatal("postgres driver not registered")
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
			name:   "prefer_simple_protocol",
			mutate: func(c *dbm.Config) { c.PreferSimpleProtocol = true },
			wants:  []string{"extra=prefer_simple_protocol:true"},
		},
		{
			name:   "without_quoting_check",
			mutate: func(c *dbm.Config) { c.WithoutQuotingCheck = true },
			wants:  []string{"extra=without_quoting_check:true"},
		},
		{
			name:   "without_returning",
			mutate: func(c *dbm.Config) { c.WithReturningDisabled = true },
			wants:  []string{"extra=without_returning:true"},
		},
		{
			name: "all_three",
			mutate: func(c *dbm.Config) {
				c.PreferSimpleProtocol = true
				c.WithoutQuotingCheck = true
				c.WithReturningDisabled = true
			},
			wants: []string{
				"prefer_simple_protocol:true",
				"without_quoting_check:true",
				"without_returning:true",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := pgConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := pgConfig.BuildDSN(conf)
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

// TestPostgresExtrasOpen verifies that the Open function in driver_postgres.go
// returns a non-nil gorm.Dialector for both code paths: the extras branch
// (postgres.New(cfg) with cleaned DSN) and the zero-config fast path
// (postgres.Open(dsn) without extras).
func TestPostgresExtrasOpen(t *testing.T) {
	pgConfig, ok := dbm.GetDriver("postgres")
	if !ok {
		t.Fatal("postgres driver not registered")
	}

	// Path 1: with extras
	conf := pgConfig.DefaultConfig
	conf.PreferSimpleProtocol = true
	conf.WithoutQuotingCheck = true
	conf.WithReturningDisabled = true
	dsn := pgConfig.BuildDSN(conf)
	if !strings.Contains(dsn, "extra=") {
		t.Fatalf("expected DSN to have 'extra=...', got: %s", dsn)
	}
	dialector := pgConfig.Open(dsn)
	if dialector == nil {
		t.Errorf("Open returned nil dialector for DSN with extras")
	}

	// Path 2: zero-config fast path
	conf2 := pgConfig.DefaultConfig
	dsn2 := pgConfig.BuildDSN(conf2)
	if strings.Contains(dsn2, "extra=") {
		t.Fatalf("expected DSN to NOT have 'extra=', got: %s", dsn2)
	}
	dialector2 := pgConfig.Open(dsn2)
	if dialector2 == nil {
		t.Errorf("Open returned nil dialector for DSN without extras (fast path)")
	}

	// Path 3: single flag (covers individual switch cases)
	conf3 := pgConfig.DefaultConfig
	conf3.WithoutQuotingCheck = true
	dsn3 := pgConfig.BuildDSN(conf3)
	dialector3 := pgConfig.Open(dsn3)
	if dialector3 == nil {
		t.Errorf("Open returned nil dialector for DSN with only without_quoting_check")
	}
}

// TestPostgresExtrasOpenNonTrueValues covers the `if fv != "true" { skip }`
// branch in driver_postgres.go where extras contain non-"true" values.
func TestPostgresExtrasOpenNonTrueValues(t *testing.T) {
	pgConfig, ok := dbm.GetDriver("postgres")
	if !ok {
		t.Fatal("postgres driver not registered")
	}

	dsn := pgConfig.BuildDSN(pgConfig.DefaultConfig) + " extra=prefer_simple_protocol:false\x1Fwithout_quoting_check:true"
	dialector := pgConfig.Open(dsn)
	if dialector == nil {
		t.Errorf("Open returned nil for DSN with non-'true' extras")
	}
}
