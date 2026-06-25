package dbm

import (
	"reflect"
	"testing"

	sqliteDriver "github.com/glebarez/sqlite"
	"gorm.io/gorm"
) // TestRegisterDriver asserts that a builder passed to RegisterDriver is
// retrievable verbatim via GetDriver under the same dialect key.
func TestRegisterDriver(t *testing.T) {
	name := "dbm_test_register_marker"

	expected := ConnectionBuilder{
		BuildDSN: func(c Config) string { return c.Name },
		Open: func(dsn string) gorm.Dialector {
			return sqliteDriver.Open(dsn)
		},
		DefaultConfig: Config{
			ConnName: name,
			Name:     "x",
		},
	}

	RegisterDriver(name, expected)

	got, ok := GetDriver(name)
	if !ok {
		t.Fatalf("expected driver %q to be registered", name)
	}
	if got.BuildDSN == nil {
		t.Error("BuildDSN should be non-nil after registration")
	}
	if got.Open == nil {
		t.Error("Open should be non-nil after registration")
	}
	if !reflect.DeepEqual(got.DefaultConfig, expected.DefaultConfig) {
		t.Errorf("DefaultConfig: want %+v got %+v", expected.DefaultConfig, got.DefaultConfig)
	}
	if dsn := got.BuildDSN(Config{Name: "hello"}); dsn != "hello" {
		t.Errorf("BuildDSN output: want %q got %q", "hello", dsn)
	}
}

// TestGetDriverUnknown asserts that an unknown dialect yields (zero, false)
// from GetDriver: ok=false and the returned builder is the zero value.
func TestGetDriverUnknown(t *testing.T) {
	const missingName = "dbm_test_unknown_marker_xyz"
	b, ok := GetDriver(missingName)
	if ok {
		t.Error("ok should be false for unknown driver")
	}
	if b.BuildDSN != nil {
		t.Error("BuildDSN should be nil for unknown driver")
	}
	if b.Open != nil {
		t.Error("Open should be nil for unknown driver")
	}
	if !reflect.DeepEqual(b.DefaultConfig, Config{}) {
		t.Errorf("DefaultConfig should be zero value, got: %+v", b.DefaultConfig)
	}
}
