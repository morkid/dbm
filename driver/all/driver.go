// Package all bundles every built-in driver shipped in this
// repository behind a single side-effect import. Import it as
// `import _ "github.com/morkid/dbm/driver/all"` to register all
// seven drivers at once; import the subpackages individually (e.g.
// `driver/sqlite`) if you want a smaller binary.
package all

import (
	_ "github.com/morkid/dbm/driver/clickhouse"
	_ "github.com/morkid/dbm/driver/cockroach"
	_ "github.com/morkid/dbm/driver/mssql"
	_ "github.com/morkid/dbm/driver/mysql"
	_ "github.com/morkid/dbm/driver/postgres"
	_ "github.com/morkid/dbm/driver/sqlite"
	_ "github.com/morkid/dbm/driver/tidb"
)
