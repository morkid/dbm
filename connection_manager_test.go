package dbm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	sqliteDriver "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils"
)

func init() {
	// Register sqlite driver inline to avoid circular import.
	// The driver/sqlite sub-package imports dbm, so importing it
	// from a dbm internal test file would create an import cycle.
	RegisterDriver("sqlite", ConnectionBuilder{
		BuildDSN: func(c Config) string {
			return "file:" + c.Name + "?cache=shared"
		},
		Open: sqliteDriver.Open,
		DefaultConfig: Config{
			ConnName:     "sqlite",
			Name:         ":memory:",
			MaxLifeTime:  3600,
			MaxIdleTime:  300,
			MaxIdleConns: 1,
			MaxOpenConns: 2,
		},
	})
}

type DemoModel struct {
	gorm.Model
	Name string
}

type InvalidIndex1 struct {
	gorm.Model
	Name string `gorm:"index:NAME"`
}

type InvalidIndex2 struct {
	gorm.Model
	Name string `gorm:"index:NAME"`
}

type InvalidModel struct {
	gorm.Model
	Name any `gorm:"index:NAME"`
}

var SeedSample2 Seed = func(tx *gorm.DB) error {
	return tx.Create(&DemoModel{}).Error
}

type SeedSample struct{}

func (s *SeedSample) Seed(tx *gorm.DB) error {
	return SeedSample2(tx)
}

func TestConnectionManager(t *testing.T) {
	password := url.QueryEscape("P@ssw0rd!:$#")
	uri, e := url.Parse(fmt.Sprintf("mysql://username:%s@localhost:9000/example?charset=utf8mb4&parseTime=True&loc=Local", password))
	fmt.Println(e)
	jsonURI, e := json.MarshalIndent(uri, "", "  ")
	fmt.Println(string(jsonURI), e)
	fmt.Println(uri.User.Username())
	pass, ok := uri.User.Password()
	fmt.Println(pass, ok, password)

	OnConnectionCreated(func(name string, db *gorm.DB) {
		if utils.AssertEqual(name, "") && utils.AssertEqual(db, nil) {
			t.Error("Empty name or db")
			t.FailNow()
		}
	})
	dbm := New()
	c1 := "c1"
	dbm.Register(c1, Config{
		AutoMigrate:    true,
		MigrationItems: []any{&DemoModel{}},
		MigrationSeeds: []any{&DemoModel{}, SeedSample2, &SeedSample{}},
	})
	_, err := dbm.Connect(c1)
	if !utils.AssertEqual(err, nil) {
		t.Error(err)
		t.FailNow()
	}

	_, err = dbm.Connect("example")
	if utils.AssertEqual(err == nil, true) {
		t.FailNow()
	}

	func() {
		defer func() {
			if recover() != nil {
				t.Error("GetDefault should not return panic if no default connection")
				t.FailNow()
			}
		}()

		db := dbm.GetDefault()
		if utils.AssertEqual(db, nil) {
			t.Error("GetDefault not returning value")
			t.FailNow()
		}
	}()

	func() {
		defer func() {
			if recover() == nil {
				t.Error("GetDefault must be return panic if no default connection")
				t.FailNow()
			}
		}()

		dbm.SetDefault("unavailable")
		dbm.GetDefault()
	}()

	dbm2 := New()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("GetDefault must be return panic if no default connection")
				t.FailNow()
			}
		}()

		dbm2.GetDefault()
	}()

	err = dbm2.Register("example", Config{}, true)
	if !utils.AssertEqual(err, nil) {
		t.Error("auto connect on register should be success")
		t.FailNow()
	}

	dbm2.Register("unavailabledriver", Config{Type: "unavailabledriver"})
	_, err = dbm2.Connect("unavailabledriver")
	if utils.AssertEqual(err, nil) {
		t.Error("unavailabledriver should be error")
		t.FailNow()
	}

	_, err = dbm2.(*connectionManager).createDialect("notavailable")
	if utils.AssertEqual(err, nil) {
		t.Error("notavailable should be error")
		t.FailNow()
	}

}

func TestDefaultConnection(t *testing.T) {
	dbm := New()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("GetDefault must be return panic if no default connection")
				t.FailNow()
			}
		}()

		dbm.SetDefault("unknown")
		dbm.GetDefault()
	}()
}

func TestInvalidIndex(t *testing.T) {
	dbm := New()
	dbm.Register("c1", Config{
		AutoMigrate:    true,
		MigrationItems: []any{&InvalidIndex1{}, &InvalidIndex2{}, &InvalidModel{}},
	})

	err, _ := dbm.Connect("c1")
	if utils.AssertEqual(err, nil) {
		t.Error("Connect should be error")
		t.FailNow()
	}
}

func TestConnectWithLogLevel(t *testing.T) {
	dbm := New()
	dbm.Register("loglevel_silent", Config{LogLevel: "silent"})
	_, err := dbm.Connect("loglevel_silent")
	if err != nil {
		t.Error(err)
	}

	dbm2 := New()
	dbm2.Register("loglevel_error", Config{LogLevel: "error"})
	_, err = dbm2.Connect("loglevel_error")
	if err != nil {
		t.Error(err)
	}

	dbm3 := New()
	dbm3.Register("loglevel_info", Config{LogLevel: "info"})
	_, err = dbm3.Connect("loglevel_info")
	if err != nil {
		t.Error(err)
	}

	dbm4 := New()
	dbm4.Register("loglevel_warn", Config{LogLevel: "warn"})
	_, err = dbm4.Connect("loglevel_warn")
	if err != nil {
		t.Error(err)
	}
}

func TestConnectWithSlowThreshold(t *testing.T) {
	dbm := New()
	dbm.Register("slow", Config{LogSlowThreshold: 500})
	_, err := dbm.Connect("slow")
	if err != nil {
		t.Error(err)
	}
}

func TestConnectWithCustomLogger(t *testing.T) {
	dbm := New()
	dbm.Register("custom_log", Config{Logger: logger.Default.LogMode(logger.Silent)})
	_, err := dbm.Connect("custom_log")
	if err != nil {
		t.Error(err)
	}
}

func TestConnectWithPlugins(t *testing.T) {
	dbm := New()
	dbm.Register("plugins", Config{Plugins: []gorm.Plugin{&testPlugin{}}})
	_, err := dbm.Connect("plugins")
	if err != nil {
		t.Error(err)
	}

	dbm2 := New()
	dbm2.Register("plugins_fail", Config{Plugins: []gorm.Plugin{&testPlugin{fail: true}}})
	_, _ = dbm2.Connect("plugins_fail")
}

type testPlugin struct{ fail bool }

func (p *testPlugin) Name() string { return "test" }
func (p *testPlugin) Initialize(db *gorm.DB) error {
	if p.fail {
		return fmt.Errorf("plugin init failed")
	}
	return nil
}

func TestConnectWithNamingStrategy(t *testing.T) {
	dbm := New()
	dbm.Register("custom_naming", Config{
		NamingStrategy: &schema.NamingStrategy{
			TablePrefix:   "app_",
			SingularTable: false,
		},
	})
	_, err := dbm.Connect("custom_naming")
	if err != nil {
		t.Error(err)
	}

	dbm2 := New()
	dbm2.Register("no_singular", Config{
		TablePrefix:           "tbl_",
		SingularTableDisabled: true,
	})
	_, err = dbm2.Connect("no_singular")
	if err != nil {
		t.Error(err)
	}
}

func TestConnectWithCreateBatchSize(t *testing.T) {
	dbm := New()
	dbm.Register("batch", Config{CreateBatchSize: 100})
	_, err := dbm.Connect("batch")
	if err != nil {
		t.Error(err)
	}
}

func TestConnectWithDryRun(t *testing.T) {
	dbm := New()
	dbm.Register("dryrun", Config{DryRun: true})
	db, err := dbm.Connect("dryrun")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.DryRun, true) {
		t.Error("DryRun should be true")
		t.FailNow()
	}
}

func TestConnectWithSkipDefaultTransaction(t *testing.T) {
	dbm := New()
	dbm.Register("skip_tx", Config{SkipDefaultTransaction: true})
	db, err := dbm.Connect("skip_tx")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.SkipDefaultTransaction, true) {
		t.Error("SkipDefaultTransaction should be true")
		t.FailNow()
	}
}

func TestConnectWithTranslateError(t *testing.T) {
	dbm := New()
	dbm.Register("translate_err", Config{TranslateError: true})
	db, err := dbm.Connect("translate_err")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.TranslateError, true) {
		t.Error("TranslateError should be true")
		t.FailNow()
	}
}

func TestConnectWithNowFunc(t *testing.T) {
	stamp := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	dbm := New()
	dbm.Register("now", Config{NowFunc: func() time.Time { return stamp }})
	db, err := dbm.Connect("now")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if db.Config.NowFunc == nil {
		t.Error("NowFunc should be set")
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.NowFunc(), stamp) {
		t.Error("NowFunc should return the configured stamp")
		t.FailNow()
	}
}

func TestConnectWithAutoPingDisabled(t *testing.T) {
	dbm := New()
	dbm.Register("nopings", Config{AutoPingDisabled: true})
	db, err := dbm.Connect("nopings")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.DisableAutomaticPing, true) {
		t.Error("DisableAutomaticPing should be true")
		t.FailNow()
	}
}

func TestConnectWithAllowGlobalUpdate(t *testing.T) {
	dbm := New()
	dbm.Register("glb", Config{AllowGlobalUpdate: true})
	db, err := dbm.Connect("glb")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.AllowGlobalUpdate, true) {
		t.Error("AllowGlobalUpdate should be true")
		t.FailNow()
	}
}

func TestConnectWithQueryFields(t *testing.T) {
	dbm := New()
	dbm.Register("qf", Config{QueryFields: true})
	db, err := dbm.Connect("qf")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.QueryFields, true) {
		t.Error("QueryFields should be true")
		t.FailNow()
	}
}

func TestConnectWithPrepareStmtAdvanced(t *testing.T) {
	// Verify that PrepareStmtMaxSize and PrepareStmtTTL are applied
	// to gorm.Config. The TTL field is in seconds (int) on Config and is
	// converted to time.Duration(secs) * time.Second on gorm.Config.
	dbm := New()
	dbm.Register("prepstmt_advanced", Config{
		PrepareStmt:        true,
		PrepareStmtMaxSize: 100,
		PrepareStmtTTL:     300,
	})
	db, err := dbm.Connect("prepstmt_advanced")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.PrepareStmtMaxSize, 100) {
		t.Errorf("PrepareStmtMaxSize should be 100, got: %d", db.Config.PrepareStmtMaxSize)
		t.FailNow()
	}
	if !utils.AssertEqual(db.Config.PrepareStmtTTL, 300*time.Second) {
		t.Errorf("PrepareStmtTTL should be 300s, got: %v", db.Config.PrepareStmtTTL)
		t.FailNow()
	}

	// Zero-value path: PrepareStmtMaxSize/TTL should default to zero when fields unset.
	dbmZero := New()
	dbmZero.Register("prepstmt_zero", Config{PrepareStmt: false})
	dbZero, err := dbmZero.Connect("prepstmt_zero")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(dbZero.Config.PrepareStmtMaxSize, 0) {
		t.Errorf("PrepareStmtMaxSize default should be 0, got: %d", dbZero.Config.PrepareStmtMaxSize)
		t.FailNow()
	}
	if !utils.AssertEqual(dbZero.Config.PrepareStmtTTL, time.Duration(0)) {
		t.Errorf("PrepareStmtTTL default should be 0, got: %v", dbZero.Config.PrepareStmtTTL)
		t.FailNow()
	}

	// Defensive path: negative TTL is clamped to 0 to avoid zero/negative
	// time.Duration semantics that would cause instant cache eviction.
	dbmNeg := New()
	dbmNeg.Register("prepstmt_negative_ttl", Config{
		PrepareStmt:    true,
		PrepareStmtTTL: -5,
	})
	dbNeg, err := dbmNeg.Connect("prepstmt_negative_ttl")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(dbNeg.Config.PrepareStmtTTL, time.Duration(0)) {
		t.Errorf("negative PrepareStmtTTL should be clamped to 0, got: %v", dbNeg.Config.PrepareStmtTTL)
		t.FailNow()
	}
}

func TestConnectWithKeepFKConstraints(t *testing.T) {
	// Default (zero KeepFKConstraints) preserves prior behavior:
	// DisableForeignKeyConstraintWhenMigrating == true (no FK generated).
	dbmDefault := New()
	dbmDefault.Register("fk_default", Config{})
	dbDefault, err := dbmDefault.Connect("fk_default")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(dbDefault.Config.DisableForeignKeyConstraintWhenMigrating, true) {
		t.Error("default should preserve 'no FK' behavior")
		t.FailNow()
	}

	// Explicit false behaves identically to default (zero-value path is covered).
	dbmFalse := New()
	dbmFalse.Register("fk_false", Config{KeepFKConstraints: false})
	dbFalse, err := dbmFalse.Connect("fk_false")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(dbFalse.Config.DisableForeignKeyConstraintWhenMigrating, true) {
		t.Error("explicit KeepFKConstraints=false should preserve no-FK behavior")
		t.FailNow()
	}

	// KeepFKConstraints=true opts in to FK generation.
	dbmOptIn := New()
	dbmOptIn.Register("fk_optin", Config{KeepFKConstraints: true})
	dbOptIn, err := dbmOptIn.Connect("fk_optin")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !utils.AssertEqual(dbOptIn.Config.DisableForeignKeyConstraintWhenMigrating, false) {
		t.Error("KeepFKConstraints=true should enable FK generation")
		t.FailNow()
	}
}
