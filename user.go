package main

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	HashedPassword string
	Username       string
	Email          string
}
