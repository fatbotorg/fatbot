package main

import (
	"errors"
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
}

func getDB() string {
	path := os.Getenv("DBPATH")
	if path == "" {
		return "fat.db"
	}
	return path
}

func getUsers() []User {
	db, err := gorm.Open(sqlite.Open(getDB()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	var users []User
	db.Find(&users)
	return users
}

func getUser(message *tgbotapi.Message) User {
	db, err := gorm.Open(sqlite.Open(getDB()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	var user User
	db.Where(User{
		Username: message.From.UserName,
		Name:     message.From.FirstName,
		// ChatID:         message.Chat.ID,
		TelegramUserID: message.From.ID,
	}).FirstOrCreate(&user)
	if user.ChatID == user.TelegramUserID || user.ChatID == 0 {
		db.Model(&user).Where("username = ?", user.Username).Update("chat_id", message.Chat.ID)
	}
	return user
}

func updateWorkout(username string, daysago int64) error {
	db, err := gorm.Open(sqlite.Open(getDB()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	var user User
	db.Where("username = ?", username).Find(&user)
	db.Model(&user).Where("username = ?", username).Update("last_last_workout", user.LastWorkout)
	when := time.Now()
	if daysago != 0 {
		when = time.Now().Add(time.Duration(-24*daysago) * time.Hour)
	}
	db.Model(&user).Where("username = ?", username).Update("last_workout", when)

	return nil
}

func updateUserInactive(username string) {
	db, err := gorm.Open(sqlite.Open(getDB()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	var user User
	db.Model(&user).Where("username = ?", username).Update("is_active", false)
	db.Model(&user).Where("username = ?", username).Update("was_notified", false)
}

func rollbackLastWorkout(username string) error {
	db, err := gorm.Open(sqlite.Open(getDB()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	var user User
	log.Info(username)
	if err := db.Where(User{Username: username}).First(&user).Error; err != nil {
		log.Error(err)
		return err
	} else {
		db.Model(&user).Where("username = ?", username).Update("last_workout", user.LastLastWorkout)
	}
	return nil
}

func updateUserImage(username string) error {
	db, err := gorm.Open(sqlite.Open(getDB()), &gorm.Config{})
	if err != nil {
		return errors.New("failed to connect database")
	}
	var user User
	db.Model(&user).Where("username = ?", username).Update("photo_update", time.Now())
	return nil
}

func db() {
	db, err := gorm.Open(sqlite.Open(getDB()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&User{})
}
