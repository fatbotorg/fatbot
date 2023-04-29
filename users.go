package main

import (
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

func (user *User) sendPrivateMessage(bot *tgbotapi.BotAPI, text string) error {
	msg := tgbotapi.NewMessage(user.TelegramUserID, text)
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (user *User) getName() (name string) {
	if user.NickName != "" {
		name = user.NickName
	} else {
		name = user.Name
	}
	return
}

func getUserById(userId int64) (user User, err error) {
	db := getDB()
	if err := db.Model(&user).Where("telegram_user_id = ?", userId).Find(&user).Error; err != nil {
		return user, err
	}
	return user, nil
}

func getUserFromMessage(message *tgbotapi.Message) User {
	db := getDB()
	var user User
	db.Where(User{
		Username:       message.From.UserName,
		Name:           message.From.FirstName,
		TelegramUserID: message.From.ID,
	}).FirstOrCreate(&user)
	if user.ChatID == user.TelegramUserID || user.ChatID == 0 {
		db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("chat_id", message.Chat.ID)
	}
	return user
}

func updateUserInactive(userId int64) {
	db := getDB()
	var user User
	db.Model(&user).Where("telegram_user_id = ?", userId).Update("is_active", false)
	db.Model(&user).Where("telegram_user_id = ?", userId).Update("was_notified", false)
}

func renameUser(userId int64, name string) error {
	db := getDB()
	var user User
	if err := db.Model(&user).Where("telegram_user_id = ?", userId).Update("nick_name", name).Error; err != nil {
		return err
	}
	return nil
}
