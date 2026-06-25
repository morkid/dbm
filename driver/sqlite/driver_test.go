package sqlite

import (
	"strings"
	"testing"

	"github.com/morkid/dbm"
)

func TestSqliteConfig(t *testing.T) {
	sqliteConfig, ok := dbm.GetDriver("sqlite")
	if !ok {
		t.Fatal("sqlite driver not registered")
	}
	conf := sqliteConfig.DefaultConfig

	dsn := sqliteConfig.BuildDSN(conf)
	t.Log(dsn)
}

// TestSqliteDSNParams verifies that driver_sqlite.go BuildDSN appends the
// new SQLiteCache and SQLitePragmas DSN parameters when set, produces
// correct modernc.org/sqlite `_pragma=key(value)` pairs, and preserves
// ExtraParams passthrough.
func TestSqliteDSNParams(t *testing.T) {
	sqliteConfig, ok := dbm.GetDriver("sqlite")
	if !ok {
		t.Fatal("sqlite driver not registered")
	}

	cases := []struct {
		name   string
		mutate func(*dbm.Config)
		want   []string
	}{
		{
			name:   "cache_private",
			mutate: func(c *dbm.Config) { c.SQLiteCache = "private" },
			want:   []string{"cache=private"},
		},
		{
			name:   "cache_explicit_shared",
			mutate: func(c *dbm.Config) { c.SQLiteCache = "shared" },
			want:   []string{"cache=shared"},
		},
		{
			name: "single_pragma",
			mutate: func(c *dbm.Config) {
				c.SQLitePragmas = map[string]string{"journal_mode": "WAL"}
			},
			want: []string{"_pragma=journal_mode%28WAL%29"},
		},
		{
			name: "multiple_pragmas",
			mutate: func(c *dbm.Config) {
				c.SQLitePragmas = map[string]string{
					"journal_mode": "WAL",
					"busy_timeout": "5000",
				}
			},
			want: []string{
				"_pragma=journal_mode%28WAL%29",
				"_pragma=busy_timeout%285000%29",
			},
		},
		{
			name: "cache_and_pragma",
			mutate: func(c *dbm.Config) {
				c.SQLiteCache = "private"
				c.SQLitePragmas = map[string]string{"foreign_keys": "on"}
			},
			want: []string{
				"cache=private",
				"_pragma=foreign_keys%28on%29",
			},
		},
		{
			name:   "extra_params_passthrough",
			mutate: func(c *dbm.Config) { c.ExtraParams = "foo=bar" },
			want:   []string{"foo=bar"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := sqliteConfig.DefaultConfig
			tc.mutate(&conf)
			dsn := sqliteConfig.BuildDSN(conf)
			t.Log(dsn)
			for _, want := range tc.want {
				if !strings.Contains(dsn, want) {
					t.Errorf("expected DSN to contain %q, got: %s", want, dsn)
				}
			}
		})
	}
}

// TestSqliteConnectInMemory smoke-tests the SQLite Connect end-to-end with
// the in-memory default.
func TestSqliteConnectInMemory(t *testing.T) {
	mgr := dbm.New()
	mgr.Register("sqlite_inmem", dbm.Config{
		SQLitePragmas: map[string]string{"foreign_keys": "on"},
	})
	_, err := mgr.Connect("sqlite_inmem")
	if err != nil {
		t.Error(err)
	}
}
