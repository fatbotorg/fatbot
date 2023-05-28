package users

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SendMessageToAdmins(bot *tgbotapi.BotAPI, message tgbotapi.MessageConfig) {
	admins := GetAdminUsers()
	for _, admin := range admins {
		admin.SendPrivateMessage(bot, message)
	}
}
