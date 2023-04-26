package main

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BUG:
// returns the entire DB, needs filtering by chat_id
func getUsers() []User {
	db := getDB()
	var users []User
	db.Find(&users)
	return users
}

func getUser(message *tgbotapi.Message) User {
	db := getDB()
	var user User
	db.Where(User{
		Username: message.From.UserName,
		Name:     message.From.FirstName,
		// ChatID:         message.Chat.ID,
		TelegramUserID: message.From.ID,
	}).FirstOrCreate(&user)
	if user.ChatID == user.TelegramUserID || user.ChatID == 0 {
		db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("chat_id", message.Chat.ID)
	}
	return user
}

func updateUserInactive(user_id int64) {
	db := getDB()
	var user User
	db.Model(&user).Where("telegram_user_id = ?", user_id).Update("is_active", false)
	db.Model(&user).Where("telegram_user_id = ?", user_id).Update("was_notified", false)
}

func updateUserImage(user_id int64) error {
	db := getDB()
	var user User
	db.Model(&user).Where("telegram_user_id = ?", user_id).Update("photo_update", time.Now())
	return nil
}
