package cockroach

import (
	"net/url"
	"strings"
	"testing"

	"github.com/morkid/dbm"
)

func TestCockroachConfig(t *testing.T) {
	crConfig, ok := dbm.GetDriver("cockroach")
	if !ok {
		t.Fatal("cockroach driver not registered")
	}
	conf := crConfig.DefaultConfig
	conf.FromDSN("cockroach://user:pass@localhost/dbname?sslmode=disable&extra=1")
	conf.Timezone = url.QueryEscape("Asia/Jakarta")

	dsn := crConfig.BuildDSN(conf)
	t.Log(dsn)
}

func TestCockroachAppName(t *testing.T) {
	crConfig, ok := dbm.GetDriver("cockroach")
	if !ok {
		t.Fatal("cockroach driver not registered")
	}

	// With explicit AppName.
	conf := crConfig.DefaultConfig
	conf.AppName = "CockroachService"
	conf.ConnName = "ignored"
	dsn := crConfig.BuildDSN(conf)
	t.Log(dsn)
	if !strings.Contains(dsn, "application_name=cockroachservice") {
		t.Errorf("expected application_name=cockroachservice, got: %s", dsn)
	}

	// Without AppName: fallback to conf.ConnName.
	confFallback := crConfig.DefaultConfig
	confFallback.AppName = ""
	confFallback.ConnName = "fallback-name"
	dsnFallback := crConfig.BuildDSN(confFallback)
	t.Log(dsnFallback)
	if !strings.Contains(dsnFallback, "application_name=fallback-name") {
		t.Errorf("expected fallback application_name=fallback-name, got: %s", dsnFallback)
	}
}

func TestCockroachDSNParams(t *testing.T) {
	crConfig, ok := dbm.GetDriver("cockroach")
	if !ok {
		t.Fatal("cockroach driver not registered")
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
			conf := crConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := crConfig.BuildDSN(conf)
			t.Log(dsn)
			if !strings.Contains(dsn, tc.want) {
				t.Errorf("expected DSN to contain %q, got: %s", tc.want, dsn)
			}
		})
	}
}

func TestCockroachExtrasBuildDSN(t *testing.T) {
	crConfig, ok := dbm.GetDriver("cockroach")
	if !ok {
		t.Fatal("cockroach driver not registered")
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
			conf := crConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := crConfig.BuildDSN(conf)
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

func TestCockroachExtrasOpen(t *testing.T) {
	crConfig, ok := dbm.GetDriver("cockroach")
	if !ok {
		t.Fatal("cockroach driver not registered")
	}

	// Path 1: with extras
	conf := crConfig.DefaultConfig
	conf.PreferSimpleProtocol = true
	conf.WithoutQuotingCheck = true
	conf.WithReturningDisabled = true
	dsn := crConfig.BuildDSN(conf)
	if !strings.Contains(dsn, "extra=") {
		t.Fatalf("expected DSN to have 'extra=...', got: %s", dsn)
	}
	dialector := crConfig.Open(dsn)
	if dialector == nil {
		t.Errorf("Open returned nil dialector for DSN with extras")
	}

	// Path 2: zero-config fast path
	conf2 := crConfig.DefaultConfig
	dsn2 := crConfig.BuildDSN(conf2)
	if strings.Contains(dsn2, "extra=") {
		t.Fatalf("expected DSN to NOT have 'extra=', got: %s", dsn2)
	}
	dialector2 := crConfig.Open(dsn2)
	if dialector2 == nil {
		t.Errorf("Open returned nil dialector for DSN without extras (fast path)")
	}

	// Path 3: single flag (covers individual switch cases)
	conf3 := crConfig.DefaultConfig
	conf3.WithoutQuotingCheck = true
	dsn3 := crConfig.BuildDSN(conf3)
	dialector3 := crConfig.Open(dsn3)
	if dialector3 == nil {
		t.Errorf("Open returned nil dialector for DSN with only without_quoting_check")
	}
}

func TestCockroachExtrasOpenNonTrueValues(t *testing.T) {
	crConfig, ok := dbm.GetDriver("cockroach")
	if !ok {
		t.Fatal("cockroach driver not registered")
	}

	dsn := crConfig.BuildDSN(crConfig.DefaultConfig) + " extra=prefer_simple_protocol:false\x1Fwithout_quoting_check:true"
	dialector := crConfig.Open(dsn)
	if dialector == nil {
		t.Errorf("Open returned nil for DSN with non-'true' extras")
	}
}

func TestCockroachConnectCoveredCases(t *testing.T) {
	mgr := dbm.New()
	mgr.Register("cr_fallback", dbm.Config{
		Type: "cockroach",
		Host: "127.0.0.1",
		Port: "26257",
		User: "root",
		Pass: "root",
		Name: "defaultdb",
	})
	_, _ = mgr.Connect("cr_fallback") // expected to fail; not asserted
}
