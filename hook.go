package dbm

import "gorm.io/gorm"

// ConnectionCallback is a callback function that is called when a connection is created
type ConnectionCallback func(name string, db *gorm.DB)

var onConnectionCreates []ConnectionCallback

// OnConnectionCreated registers a callback function that is called when a connection is created
func OnConnectionCreated(callback ConnectionCallback) {
	onConnectionCreates = append(onConnectionCreates, callback)
}
