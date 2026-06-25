package dbm

import "gorm.io/gorm"

// ConnectionCallback is a callback function that is called when a connection is created
type ConnectionCallback func(name string, db *gorm.DB)

var onConnectionCreates []ConnectionCallback

// OnConnectionCreated registers a callback function that is called when a
// connection is created.
//
// Deprecated: prefer the chainable, per-instance method on Connection:
//
//	mgr := dbm.New()
//	mgr.OnConnectionCreated(callback)
//
// This global form is shared by every Connection in the process, which
// complicates test isolation. It is retained for backward compatibility
// and will be removed in a future major release. Passing a nil callback
// is a no-op (no error, no registration).
func OnConnectionCreated(callback ConnectionCallback) {
	if callback == nil {
		return
	}
	onConnectionCreates = append(onConnectionCreates, callback)
}
