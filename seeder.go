package dbm

import (
	"gorm.io/gorm"
)

// Seed is a function that seeds the database
// It receives a gorm database instance and returns an error if the seeding fails
type Seed func(db *gorm.DB) error

// Seeder is an interface for seeding the database
// It has a Seed method that receives a gorm database instance and returns an error if the seeding fails
type Seeder interface {
	Seed(db *gorm.DB) error
}
