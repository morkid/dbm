package dbm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
)

func TestConfig(t *testing.T) {
	dsn := "mysql://user:pass@/mysql?name=mysql&charset=utf8&timezone=Asia/Jakarta&extra=1"
	config := new(Config)
	config.FromDSN(dsn)

	out, _ := json.MarshalIndent(config, "", "  ")
	qq, _ := url.ParseQuery(config.ExtraParams)
	fmt.Println(string(out), config.ExtraParams, qq.Encode())

	dsn = "sqlite:///database.db?name="
	config = new(Config)
	err := config.FromDSN(dsn)
	if err != nil {
		t.Log(err)
	}
	out, _ = json.MarshalIndent(config, "", "  ")
	fmt.Println(string(out))
}

// TestFromDSNPortDefaults asserts the per-scheme default ports applied when
// the DSN omits an explicit :port segment.
func TestFromDSNPortDefaults(t *testing.T) {
	cases := []struct {
		scheme string
		want   string
	}{
		{"mysql", "3306"},
		{"postgres", "5432"},
		{"postgresql", "5432"},
		{"cockroach", "26257"},
		{"tidb", "4000"},
	}
	for _, tc := range cases {
		t.Run(tc.scheme, func(t *testing.T) {
			c := new(Config)
			dsn := fmt.Sprintf("%s://u:p@/db?name=%s", tc.scheme, tc.scheme)
			if err := c.FromDSN(dsn); err != nil {
				t.Fatalf("FromDSN: %v", err)
			}
			if c.Port != tc.want {
				t.Errorf("port: want %q got %q", tc.want, c.Port)
			}
		})
	}
}

// TestFromDSNQueryParams asserts each individual query-param case inside
// FromDSN's switch: log_level, log_not_found, table_prefix, and the four
// connection-pool numeric params, plus auto_migrate.
func TestFromDSNQueryParams(t *testing.T) {
	c := new(Config)
	dsn := "mysql://u:p@h/d?name=cfg" +
		"&log_level=info" +
		"&log_not_found=true" +
		"&table_prefix=p_" +
		"&max_open_conns=10" +
		"&max_idle_conns=5" +
		"&max_idle_time=60" +
		"&max_life_time=600" +
		"&auto_migrate=true"
	if err := c.FromDSN(dsn); err != nil {
		t.Fatalf("FromDSN: %v", err)
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel: want %q got %q", "info", c.LogLevel)
	}
	if !c.LogNotFound {
		t.Error("LogNotFound should be true")
	}
	if c.TablePrefix != "p_" {
		t.Errorf("TablePrefix: want %q got %q", "p_", c.TablePrefix)
	}
	if c.MaxOpenConns != 10 {
		t.Errorf("MaxOpenConns: want %d got %d", 10, c.MaxOpenConns)
	}
	if c.MaxIdleConns != 5 {
		t.Errorf("MaxIdleConns: want %d got %d", 5, c.MaxIdleConns)
	}
	if c.MaxIdleTime != 60 {
		t.Errorf("MaxIdleTime: want %d got %d", 60, c.MaxIdleTime)
	}
	if c.MaxLifeTime != 600 {
		t.Errorf("MaxLifeTime: want %d got %d", 600, c.MaxLifeTime)
	}
	if !c.AutoMigrate {
		t.Error("AutoMigrate should be true")
	}
}

// TestFromDSNSSLMode asserts the non-empty sslmode path: the value is
// stored as-is on Config.SSLMode without triggering the fallback.
func TestFromDSNSSLMode(t *testing.T) {
	c := new(Config)
	if err := c.FromDSN("mysql://u:p@h/d?name=m&sslmode=require"); err != nil {
		t.Fatalf("FromDSN: %v", err)
	}
	if c.SSLMode != "require" {
		t.Errorf("SSLMode: want %q got %q", "require", c.SSLMode)
	}
}

// TestFromDSNSSLModeEmptyFallback asserts the ?sslmode= (empty value) path
// which writes the canonical "disable" sentinel into Config.SSLMode.
func TestFromDSNSSLModeEmptyFallback(t *testing.T) {
	c := new(Config)
	if err := c.FromDSN("mysql://u:p@h/d?name=m&sslmode="); err != nil {
		t.Fatalf("FromDSN: %v", err)
	}
	if c.SSLMode != "disable" {
		t.Errorf("SSLMode empty fallback: want %q got %q", "disable", c.SSLMode)
	}
}

// TestFromDSNMissingName asserts that a DSN without an explicit name=*
// (including the empty value form) returns the documented error so callers
// can detect misconfiguration.
func TestFromDSNMissingName(t *testing.T) {
	c := new(Config)
	if err := c.FromDSN("mysql://u:p@h/d"); err == nil {
		t.Error("FromDSN without name=* param should return error")
	}
}
