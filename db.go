package main

import (
	"fatbot/accounts"
	"fatbot/users"
	"os"

	"github.com/charmbracelet/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
	db.AutoMigrate(&users.User{}, &accounts.Account{}, &users.Workout{})
	return nil
}

func isApprovedChatID(chatID int64) bool {
	db := getDB()
	var account accounts.Account
	result := db.Where("chat_id = ?", chatID).Find(&account)
	if result.RowsAffected == 0 {
		return false
	}
	return account.Approved
}
