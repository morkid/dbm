package clickhouse

import (
	"strings"
	"testing"

	"github.com/morkid/dbm"
)

func TestClickhouseAppNameRemoved(t *testing.T) {
	chConfig, ok := dbm.GetDriver("clickhouse")
	if !ok {
		t.Fatal("clickhouse driver not registered")
	}

	// With explicit AppName: must NOT contain appname=.
	conf := chConfig.DefaultConfig
	conf.AppName = "MetricsService"
	dsn := chConfig.BuildDSN(conf)
	t.Log(dsn)
	if strings.Contains(dsn, "appname=") {
		t.Errorf("expected DSN to NOT contain appname= (AppName removed for ClickHouse), got: %s", dsn)
	}

	// Without AppName: must NOT fall back to conf.ConnName as appname.
	confFallback := chConfig.DefaultConfig
	confFallback.AppName = ""
	confFallback.ConnName = "fallback-name"
	dsnFallback := chConfig.BuildDSN(confFallback)
	t.Log(dsnFallback)
	if strings.Contains(dsnFallback, "appname=") {
		t.Errorf("expected DSN to NOT contain appname= even with ConnName set, got: %s", dsnFallback)
	}

	// ExtraParams should still flow through.
	confExtra := chConfig.DefaultConfig
	confExtra.AppName = "MetricsService"
	confExtra.ExtraParams = "compress=true"
	dsnExtra := chConfig.BuildDSN(confExtra)
	t.Log(dsnExtra)
	if !strings.Contains(dsnExtra, "compress=true") {
		t.Errorf("expected ExtraParams compress=true to flow through, got: %s", dsnExtra)
	}
	if strings.Contains(dsnExtra, "appname=") {
		t.Errorf("expected DSN to NOT contain appname= even with ExtraParams set, got: %s", dsnExtra)
	}
}

func TestClickhouseConfig(t *testing.T) {
	clickhouseConfig, ok := dbm.GetDriver("clickhouse")
	if !ok {
		t.Fatal("clickhouse driver not registered")
	}
	conf := clickhouseConfig.DefaultConfig

	dsn := clickhouseConfig.BuildDSN(conf)
	t.Log(dsn)

	conf2 := clickhouseConfig.DefaultConfig
	conf2.ExtraParams = "compress=true"
	dsn2 := clickhouseConfig.BuildDSN(conf2)
	t.Log(dsn2)

	_ = clickhouseConfig.Open(dsn)

	// Open a DSN with query params but no extras (covers the i>=0 branch in
	// ClickHouse Open where values exists but no "extra" key is present).
	_ = clickhouseConfig.Open(dsn2)
}

// TestClickhouseExtrasBuildDSN verifies that all 5 dbm-specific ClickHouse
// flags are URL-encoded into an "extra" key on the ClickHouse DSN, and that no
// "extra" key is emitted when all flags are at their zero values.
func TestClickhouseExtrasBuildDSN(t *testing.T) {
	chConfig, ok := dbm.GetDriver("clickhouse")
	if !ok {
		t.Fatal("clickhouse driver not registered")
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
			name: "table_engine",
			mutate: func(c *dbm.Config) {
				c.ClickHouseTableEngine = "ENGINE=MergeTree() ORDER BY id"
			},
			wants: []string{"extra=table_engine%3AENGINE%3DMergeTree%28%29+ORDER+BY+id"},
		},
		{
			name: "table_engine_with_commas",
			mutate: func(c *dbm.Config) {
				c.ClickHouseTableEngine = "ENGINE=ReplicatedMergeTree('/clickhouse/tables/1/m','r')"
			},
			wants: []string{"extra=table_engine%3AENGINE%3DReplicatedMergeTree%28%27%2Fclickhouse%2Ftables%2F1%2Fm%27%2C%27r%27%29"},
		},
		{
			name: "default_compression",
			mutate: func(c *dbm.Config) {
				c.ClickHouseCompression = "ZSTD"
			},
			wants: []string{"extra=default_compression%3AZSTD"},
		},
		{
			name: "default_granularity",
			mutate: func(c *dbm.Config) {
				c.ClickHouseGranularity = 1
			},
			wants: []string{"extra=default_granularity%3A1"},
		},
		{
			name: "skip_init_version",
			mutate: func(c *dbm.Config) {
				c.SkipInitVersion = true
			},
			wants: []string{"extra=skip_init_version%3Atrue"},
		},
		{
			name: "disable_datetime_precision",
			mutate: func(c *dbm.Config) {
				c.ClickHouseDtPrecisionDisabled = true
			},
			wants: []string{"extra=disable_datetime_precision%3Atrue"},
		},
		{
			name: "all_five_combined",
			mutate: func(c *dbm.Config) {
				c.ClickHouseTableEngine = "ENGINE=MergeTree()"
				c.ClickHouseCompression = "LZ4"
				c.ClickHouseGranularity = 3
				c.SkipInitVersion = true
				c.ClickHouseDtPrecisionDisabled = true
			},
			wants: []string{
				"extra=table_engine%3AENGINE%3DMergeTree%28%29",
				"default_compression%3ALZ4",
				"default_granularity%3A3",
				"skip_init_version%3Atrue",
				"disable_datetime_precision%3Atrue",
			},
		},
		{
			name: "all_five_combined_with_comma",
			mutate: func(c *dbm.Config) {
				c.ClickHouseTableEngine = "ENGINE=ReplicatedMergeTree('/clickhouse/tables/1/m','r')"
				c.ClickHouseCompression = "ZSTD"
				c.ClickHouseGranularity = 5
				c.SkipInitVersion = true
				c.ClickHouseDtPrecisionDisabled = true
			},
			wants: []string{
				"extra=table_engine%3AENGINE%3DReplicatedMergeTree%28%27%2Fclickhouse%2Ftables%2F1%2Fm%27%2C%27r%27%29",
				"default_compression%3AZSTD",
				"default_granularity%3A5",
				"skip_init_version%3Atrue",
				"disable_datetime_precision%3Atrue",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := chConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := chConfig.BuildDSN(conf)
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

// TestClickhouseExtrasOpen verifies that the Open function in
// driver_clickhouse.go returns a non-nil gorm.Dialector for both code paths:
// the extras branch (clickhouse.New(cfg) with cleaned DSN) and the
// zero-config fast path (clickhouse.Open(dsn) without extras).
func TestClickhouseExtrasOpen(t *testing.T) {
	chConfig, ok := dbm.GetDriver("clickhouse")
	if !ok {
		t.Fatal("clickhouse driver not registered")
	}

	// Path 1: with extras (all five flags combined)
	confAll := chConfig.DefaultConfig
	confAll.ClickHouseTableEngine = "ENGINE=MergeTree()"
	confAll.ClickHouseCompression = "ZSTD"
	confAll.ClickHouseGranularity = 5
	confAll.SkipInitVersion = true
	confAll.ClickHouseDtPrecisionDisabled = true
	dsnAll := chConfig.BuildDSN(confAll)
	if !strings.Contains(dsnAll, "extra=") {
		t.Fatalf("expected DSN to have 'extra=', got: %s", dsnAll)
	}
	if dialectorAll := chConfig.Open(dsnAll); dialectorAll == nil {
		t.Errorf("Open returned nil dialector for DSN with extras")
	}

	// Path 2: zero-config fast path (no extras)
	confClean := chConfig.DefaultConfig
	dsnClean := chConfig.BuildDSN(confClean)
	if strings.Contains(dsnClean, "extra=") {
		t.Fatalf("expected clean DSN to NOT have 'extra=', got: %s", dsnClean)
	}
	if dialectorClean := chConfig.Open(dsnClean); dialectorClean == nil {
		t.Errorf("Open returned nil for clean DSN (fast path)")
	}

	// Path 3: single-flag (covers individual switch cases)
	for _, singleMutate := range []func(*dbm.Config){
		func(c *dbm.Config) { c.ClickHouseTableEngine = "ENGINE=Log" },
		func(c *dbm.Config) { c.ClickHouseCompression = "LZ4" },
		func(c *dbm.Config) { c.ClickHouseGranularity = 2 },
		func(c *dbm.Config) { c.SkipInitVersion = true },
		func(c *dbm.Config) { c.ClickHouseDtPrecisionDisabled = true },
	} {
		conf := chConfig.DefaultConfig
		singleMutate(&conf)
		dsn := chConfig.BuildDSN(conf)
		if dialector := chConfig.Open(dsn); dialector == nil {
			t.Errorf("Open returned nil for single-flag DSN: %s", dsn)
		}
	}

	// Path 4: malformed extras (non-integer default_granularity)
	cleanDSN := chConfig.BuildDSN(chConfig.DefaultConfig)
	malformedDSN := cleanDSN + "?extra=default_granularity%3Anot_a_number"
	if !strings.Contains(malformedDSN, "extra=") {
		t.Fatalf("expected malformed DSN to have 'extra=', got: %s", malformedDSN)
	}
	if dialectorMalformed := chConfig.Open(malformedDSN); dialectorMalformed == nil {
		t.Errorf("Open returned nil for malformed extras")
	}

	// Path 5: explicit empty `extra=`
	emptyDSN := chConfig.BuildDSN(chConfig.DefaultConfig) + "?extra="
	if dialectorEmpty := chConfig.Open(emptyDSN); dialectorEmpty == nil {
		t.Errorf("Open returned nil for DSN with empty extra value")
	}
}

// TestParseClickhouseExtras directly exercises parseClickhouseExtras to
// lock in regression behavior.
func TestParseClickhouseExtras(t *testing.T) {
	cases := []struct {
		name               string
		input              string
		wantEngine         string
		wantCompression    string
		wantGranularity    int
		wantSkipInitVer    bool
		wantDtPrecDisabled bool
	}{
		{
			name:       "empty_string",
			input:      "",
			wantEngine: "",
		},
		{
			name:       "single_table_engine_no_specials",
			input:      "table_engine:ENGINE=MergeTree() ORDER BY id",
			wantEngine: "ENGINE=MergeTree() ORDER BY id",
		},
		{
			name:       "table_engine_with_commas",
			input:      "table_engine:ENGINE=ReplicatedMergeTree('/clickhouse/tables/1/m','r')",
			wantEngine: "ENGINE=ReplicatedMergeTree('/clickhouse/tables/1/m','r')",
		},
		{
			name:            "default_compression",
			input:           "default_compression:ZSTD",
			wantCompression: "ZSTD",
		},
		{
			name:            "default_granularity",
			input:           "default_granularity:3",
			wantGranularity: 3,
		},
		{
			name:  "default_granularity_zero_falls_through",
			input: "default_granularity:0",
		},
		{
			name:  "malformed_granularity_falls_through",
			input: "default_granularity:not_a_number",
		},
		{
			name:            "skip_init_version_true",
			input:           "skip_init_version:true",
			wantSkipInitVer: true,
		},
		{
			name:  "skip_init_version_other_value",
			input: "skip_init_version:yes",
		},
		{
			name:               "disable_datetime_precision_true",
			input:              "disable_datetime_precision:true",
			wantDtPrecDisabled: true,
		},
		{
			name: "all_five_combined_with_comma",
			input: "table_engine:ENGINE=ReplicatedMergeTree('/p','r')" + "\x1F" +
				"default_compression:ZSTD" + "\x1F" +
				"default_granularity:3" + "\x1F" +
				"skip_init_version:true" + "\x1F" +
				"disable_datetime_precision:true",
			wantEngine:         "ENGINE=ReplicatedMergeTree('/p','r')",
			wantCompression:    "ZSTD",
			wantGranularity:    3,
			wantSkipInitVer:    true,
			wantDtPrecDisabled: true,
		},
		{
			name: "unknown_keys_silently_dropped",
			input: "table_engine:ENGINE=Log" + "\x1F" +
				"foo:bar" + "\x1F" +
				"default_compression:LZ4",
			wantEngine:      "ENGINE=Log",
			wantCompression: "LZ4",
		},
		{
			name:       "extra_empty_value_after_valid",
			input:      "table_engine:ENGINE=Log" + "\x1F",
			wantEngine: "ENGINE=Log",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := parseClickhouseExtras(tc.input)
			if cfg.DefaultTableEngineOpts != tc.wantEngine {
				t.Errorf("DefaultTableEngineOpts = %q, want %q",
					cfg.DefaultTableEngineOpts, tc.wantEngine)
			}
			if cfg.DefaultCompression != tc.wantCompression {
				t.Errorf("DefaultCompression = %q, want %q",
					cfg.DefaultCompression, tc.wantCompression)
			}
			if cfg.DefaultGranularity != tc.wantGranularity {
				t.Errorf("DefaultGranularity = %d, want %d",
					cfg.DefaultGranularity, tc.wantGranularity)
			}
			if cfg.SkipInitializeWithVersion != tc.wantSkipInitVer {
				t.Errorf("SkipInitializeWithVersion = %v, want %v",
					cfg.SkipInitializeWithVersion, tc.wantSkipInitVer)
			}
			if cfg.DisableDatetimePrecision != tc.wantDtPrecDisabled {
				t.Errorf("DisableDatetimePrecision = %v, want %v",
					cfg.DisableDatetimePrecision, tc.wantDtPrecDisabled)
			}
		})
	}
}
