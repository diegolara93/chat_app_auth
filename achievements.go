package main

import (
	"time"

	"gorm.io/gorm"
)

// todo: add a Achievements []UserAchievement `gorm:"foreignKey:UserID"` to the user struct and also make these gorm models
type Achievement struct {
	ID          string
	Name        string
	Description string
}

type UserAchievement struct {
	UserID        uint
	AchievementID string
	UnlockedAt    time.Time
}

type AchievementService interface {
	CheckCriteria(user *User, achievementID string) bool
	RewardAchievement(user *User, achievementID string) error
	GetAchievements(user *User) ([]Achievement, error)
}

type achievementService struct {
	db *gorm.DB
}

func NewAchievementService(db *gorm.DB) AchievementService {
	return &achievementService{
		db: db,
	}
}
func (s *achievementService) CheckCriteria(user *User, achievementID string) bool {
	// Check if the user meets the criteria for the achievement
	return true
}

func (s *achievementService) RewardAchievement(user *User, achievementID string) error {
	return nil
}
func (s *achievementService) GetAchievements(user *User) ([]Achievement, error) {
	return nil, nil

}
