package main

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username        string
	Name            string
	LastWorkout     time.Time
	LastLastWorkout time.Time
	PhotoUpdate     time.Time
	ChatID          int64
	TelegramUserID  int64
	WasNotified     bool
	IsActive        bool
	Workouts        []Workout
}

type Workout struct {
	gorm.Model
	Cancelled bool
	UserID    uint
}

type Account struct {
	gorm.Model
	ChatID   int64
	Approved bool
}

func getDB() *gorm.DB {
	path := os.Getenv("DBPATH")
	if path == "" {
		path = "fat.db"
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func initDB() error {
	db := getDB()
	db.AutoMigrate(&User{}, &Account{}, &Workout{})
	return nil
}

func isApprovedChatID(chatID int64) bool {
	db := getDB()
	var account Account
	result := db.Where("chat_id = ?", chatID).Find(&account)
	if result.RowsAffected == 0 {
		return false
	}
	return account.Approved
}
