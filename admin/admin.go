package admin

import (
	"fatbot/users"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SendMessageToAdmins(bot *tgbotapi.BotAPI, message tgbotapi.MessageConfig) {
	admins := users.GetAdminUsers()
	for _, admin := range admins {
		admin.SendPrivateMessage(bot, message)
	}
}

func CreateUsersKeyboard(chatId int64) tgbotapi.InlineKeyboardMarkup {
	// BUG: THIS GETS ALL USERS #7
	// use chat_id in the argument to get specific group
	users := users.GetUsers(chatId)
	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, user := range users {
		userLabel := fmt.Sprintf("%s", user.GetName())
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(userLabel, fmt.Sprint(user.TelegramUserID)))
		if len(row) == 3 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 && len(row) < 3 {
		rows = append(rows, row)
	}
	var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}
