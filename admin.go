package main

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func isAdmin(userId int) bool {
	db := getDB()
	var user User
	db.Where("telegram_user_id = ?", userId).Find(&user)
	return user.IsAdmin
}

func handleAdminDeleteLastCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	super := isAdmin(int(update.Message.From.ID))
	if super {
		users := getUsers()
		row := []tgbotapi.InlineKeyboardButton{}
		rows := [][]tgbotapi.InlineKeyboardButton{}
		for _, user := range users {
			userLabel := fmt.Sprintf("%s,%s", user.Name, user.Username)
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(userLabel, fmt.Sprint(user.TelegramUserID)))
			if len(row) == 3 {
				rows = append(rows, row)
				row = row[:0]
			}
		}
		rows = append(rows, row)
		var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
		msg.ReplyMarkup = keyboard
		msg.Text = "Pick a user"
	}
	return msg
}
