package mssql

import (
	"strings"
	"testing"

	"github.com/morkid/dbm"
)

func TestSqlServerConfig(t *testing.T) {
	sqlserverConfig, ok := dbm.GetDriver("sqlserver")
	if !ok {
		t.Fatal("sqlserver driver not registered")
	}
	conf := sqlserverConfig.DefaultConfig

	dsn := sqlserverConfig.BuildDSN(conf)
	t.Log(dsn)

	conf2 := sqlserverConfig.DefaultConfig
	conf2.ExtraParams = "app=test"
	dsn2 := sqlserverConfig.BuildDSN(conf2)
	t.Log(dsn2)

	_ = sqlserverConfig.Open(dsn)
}

func TestSqlServerSSLModeEncryption(t *testing.T) {
	ssConfig, ok := dbm.GetDriver("sqlserver")
	if !ok {
		t.Fatal("sqlserver driver not registered")
	}

	cases := []struct {
		name        string
		sslMode     string
		wantPresent bool
		wants       []string
	}{
		{"disabled_mode", "disable", true, []string{"encrypt=disabled"}},
		{"required_mode", "require", true, []string{"encrypt=true"}},
		{"skip_verify_mode", "skip-verify", true, []string{"encrypt=true", "trustservercertificate=true"}},
		{"empty_no_encrypt", "", false, nil},
		{"require_uppercase_normalized", "REQUIRE", true, []string{"encrypt=true"}},
		{"disable_trimmed_padding", "  disable  ", true, []string{"encrypt=disabled"}},
		{"unknown_value_no_op", "verify-ca", false, nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := ssConfig.DefaultConfig
			conf.SSLMode = tc.sslMode
			dsn := ssConfig.BuildDSN(conf)
			t.Log(dsn)
			if tc.wantPresent {
				for _, want := range tc.wants {
					if !strings.Contains(dsn, want) {
						t.Errorf("expected DSN to contain %q, got: %s", want, dsn)
					}
				}
			} else {
				if strings.Contains(dsn, "encrypt=") {
					t.Errorf("expected DSN to NOT contain 'encrypt=' when SSLMode is empty, got: %s", dsn)
				}
			}
		})
	}

	// Verify user-supplied ExtraParams "encrypt=..." is still respected
	// when SSLMode is empty.
	confExtra := ssConfig.DefaultConfig
	confExtra.ExtraParams = "encrypt=false"
	dsnExtra := ssConfig.BuildDSN(confExtra)
	if !strings.Contains(dsnExtra, "encrypt=false") {
		t.Errorf("expected ExtraParams 'encrypt=false' to flow through, got: %s", dsnExtra)
	}
}

func TestSqlServerAppName(t *testing.T) {
	ssConfig, ok := dbm.GetDriver("sqlserver")
	if !ok {
		t.Fatal("sqlserver driver not registered")
	}

	// With explicit AppName.
	conf := ssConfig.DefaultConfig
	conf.AppName = "MyApp"
	dsn := ssConfig.BuildDSN(conf)
	t.Log(dsn)
	if !strings.Contains(dsn, "app+name=MyApp") {
		t.Errorf("expected app+name=MyApp (url.Values form encoding) in DSN, got: %s", dsn)
	}
	if !strings.Contains(dsn, "database=master") {
		t.Errorf("expected database=master in DSN, got: %s", dsn)
	}

	// Without AppName: fallback to conf.ConnName.
	confFallback := ssConfig.DefaultConfig
	confFallback.AppName = ""
	confFallback.ConnName = "fallback"
	dsnFallback := ssConfig.BuildDSN(confFallback)
	t.Log(dsnFallback)
	if !strings.Contains(dsnFallback, "app+name=fallback") {
		t.Errorf("expected fallback app+name=fallback (url.Values form encoding) in DSN, got: %s", dsnFallback)
	}

	// ExtraParams should still flow through alongside app name.
	confExtra := ssConfig.DefaultConfig
	confExtra.AppName = "MyApp"
	confExtra.ExtraParams = "encrypt=disable"
	dsnExtra := ssConfig.BuildDSN(confExtra)
	t.Log(dsnExtra)
	if !strings.Contains(dsnExtra, "encrypt=disable") {
		t.Errorf("expected encrypt=disable to be preserved, got: %s", dsnExtra)
	}
}

// TestSqlServerBatchBDSNParams verifies the Batch B DSN-only fields:
// SSReadOnlyIntent, SSPacketSize, SSWorkstation, SSFailoverPartner,
// SSFailoverPort, SSDialTimeout, SSConnTimeout, SSKeepAlive.
func TestSqlServerBatchBDSNParams(t *testing.T) {
	ssConfig, ok := dbm.GetDriver("sqlserver")
	if !ok {
		t.Fatal("sqlserver driver not registered")
	}

	cases := []struct {
		name   string
		mutate func(*dbm.Config)
		wants  []string
	}{
		{
			name: "read_only_intent",
			mutate: func(c *dbm.Config) {
				c.SSReadOnlyIntent = true
			},
			wants: []string{"applicationintent=readonly"},
		},
		{
			name: "packet_size",
			mutate: func(c *dbm.Config) {
				c.SSPacketSize = 8192
			},
			wants: []string{"packet+size=8192"},
		},
		{
			name: "workstation_explicit",
			mutate: func(c *dbm.Config) {
				c.SSWorkstation = "workstation-01"
			},
			wants: []string{"workstation+id=workstation-01"},
		},
		{
			name: "workstation_fallback_to_name",
			mutate: func(c *dbm.Config) {
				c.ConnName = "fallback-name"
				c.SSWorkstation = ""
			},
			wants: []string{"workstation+id=fallback-name"},
		},
		{
			name: "workstation_empty_name_no_key",
			mutate: func(c *dbm.Config) {
				c.ConnName = ""
				c.SSWorkstation = ""
				c.AppName = "explicit-app"
			},
			wants: nil,
		},
		{
			name: "failover_partner_only",
			mutate: func(c *dbm.Config) {
				c.SSFailoverPartner = "secondary.example.com"
			},
			wants: []string{"failoverPartner=secondary.example.com"},
		},
		{
			name: "failover_partner_with_port",
			mutate: func(c *dbm.Config) {
				c.SSFailoverPartner = "secondary.example.com"
				c.SSFailoverPort = "1433"
			},
			wants: []string{"failoverPartner=secondary.example.com%3A1433"},
		},
		{
			name: "dial_timeout",
			mutate: func(c *dbm.Config) {
				c.SSDialTimeout = 7
			},
			wants: []string{"dial+timeout=7s"},
		},
		{
			name: "conn_timeout",
			mutate: func(c *dbm.Config) {
				c.SSConnTimeout = 45
			},
			wants: []string{"connection+timeout=45s"},
		},
		{
			name: "keepalive",
			mutate: func(c *dbm.Config) {
				c.SSKeepAlive = 60
			},
			wants: []string{"keepalive=60s"},
		},
		{
			name: "all_fields_combined",
			mutate: func(c *dbm.Config) {
				c.SSReadOnlyIntent = true
				c.SSPacketSize = 4096
				c.SSWorkstation = "ws-A"
				c.SSFailoverPartner = "mirror.example.com"
				c.SSFailoverPort = "1434"
				c.SSDialTimeout = 10
				c.SSConnTimeout = 20
				c.SSKeepAlive = 30
			},
			wants: []string{
				"applicationintent=readonly",
				"packet+size=4096",
				"workstation+id=ws-A",
				"failoverPartner=mirror.example.com%3A1434",
				"dial+timeout=10s",
				"connection+timeout=20s",
				"keepalive=30s",
			},
		},
		{
			name: "zero_mysql_fields_name_fallback_fires",
			mutate: func(c *dbm.Config) {
				c.SSReadOnlyIntent = false
				c.SSPacketSize = 0
				c.SSDialTimeout = 0
				c.SSConnTimeout = 0
				c.SSKeepAlive = 0
				c.SSFailoverPartner = ""
				c.SSWorkstation = ""
			},
			wants: []string{
				"workstation+id=sqlserver",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := ssConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := ssConfig.BuildDSN(conf)
			t.Log(dsn)
			for _, want := range tc.wants {
				if !strings.Contains(dsn, want) {
					t.Errorf("expected DSN to contain %q, got: %s", want, dsn)
				}
			}
			if tc.name == "zero_mysql_fields_name_fallback_fires" {
				negative := []string{
					"applicationintent=",
					"packet+size=",
					"failoverPartner=",
					"dial+timeout=",
					"connection+timeout=",
					"keepalive=",
				}
				for _, neg := range negative {
					if strings.Contains(dsn, neg) {
						t.Errorf("expected DSN to NOT contain %q for zero values, got: %s", neg, dsn)
					}
				}
			}
			if tc.name == "workstation_empty_name_no_key" {
				if strings.Contains(dsn, "workstation+id=") {
					t.Errorf("expected DSN to NOT contain 'workstation+id=' when both SSWorkstation and Name are empty, got: %s", dsn)
				}
			}
		})
	}
}

// TestSqlServerDefaultStringSizeExtras verifies that DefaultStringSize
// is encoded into the DSN as `extra=default_string_size:N` and that the
// Open function returns a non-nil Dialector via the sqlserver.New(cfg) path.
func TestSqlServerDefaultStringSizeExtras(t *testing.T) {
	ssConfig, ok := dbm.GetDriver("sqlserver")
	if !ok {
		t.Fatal("sqlserver driver not registered")
	}

	// Path 1: extras round-trip produces a non-nil dialector.
	conf := ssConfig.DefaultConfig
	conf.DefaultStringSize = 191
	dsn := ssConfig.BuildDSN(conf)
	if !strings.Contains(dsn, "extra=default_string_size%3A191") {
		t.Fatalf("expected DSN to contain 'extra=default_string_size%%3A191' (URL-encoded colon), got: %s", dsn)
	}
	if dialector := ssConfig.Open(dsn); dialector == nil {
		t.Errorf("Open returned nil dialector for DSN with default_string_size extras")
	}

	// Path 2: zero value should NOT add an extra key.
	confZero := ssConfig.DefaultConfig
	dsnZero := ssConfig.BuildDSN(confZero)
	if strings.Contains(dsnZero, "extra=") {
		t.Errorf("expected DSN to NOT contain 'extra=' when DefaultStringSize is 0, got: %s", dsnZero)
	}

	// Path 3: zero-config fast path (no extras at all).
	confClean := ssConfig.DefaultConfig
	dsnClean := ssConfig.BuildDSN(confClean)
	if strings.Contains(dsnClean, "extra=") {
		t.Fatalf("expected clean DSN to NOT contain 'extra=', got: %s", dsnClean)
	}
	if dialectorClean := ssConfig.Open(dsnClean); dialectorClean == nil {
		t.Errorf("Open returned nil for clean DSN (fast path)")
	}

	// Path 4: malformed extras (non-integer)
	dsnBad := strings.Replace(ssConfig.BuildDSN(ssConfig.DefaultConfig), "?", "?extra=default_string_size%3Anot_a_number&", 1)
	dialectorBad := ssConfig.Open(dsnBad)
	if dialectorBad == nil {
		t.Errorf("Open returned nil for DSN with malformed extras")
	}

	// Path 5: explicitly empty extra= value branch.
	confEmptyExtra := ssConfig.DefaultConfig
	dsnEmptyExtra := ssConfig.BuildDSN(confEmptyExtra) + "&extra="
	dialectorEmptyExtra := ssConfig.Open(dsnEmptyExtra)
	if dialectorEmptyExtra == nil {
		t.Errorf("Open returned nil for DSN with empty 'extra=' value")
	}
}
