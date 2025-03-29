package main

import (
	"time"

	"gorm.io/gorm"
)

type ChatRoom struct {
	gorm.Model
	Name            string  `json:"name"`
	OwnerID         uint    `json:"owner_id"`
	Password        string  `json:"password,omitempty"`
	HasPassword     bool    `json:"has_password"`
	MaxParticipants int     `json:"max_participants"` // Limit of 1-10 people
	Participants    []*User `gorm:"many2many:room_participants;" json:"participants,omitempty"`
	RoomID          string  `json:"room_id"`
}

type RoomParticipant struct {
	RoomID     uint `gorm:"primaryKey"`
	UserID     uint `gorm:"primaryKey"`
	JoinedAt   time.Time
	IsActive   bool // Track if user is currently in the room
	LastActive time.Time
}
