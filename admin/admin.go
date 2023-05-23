package admin

import (
	"fatbot/state"
	"fatbot/users"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SendMessageToAdmins(bot *tgbotapi.BotAPI, message tgbotapi.MessageConfig) {
	admins := users.GetAdminUsers()
	for _, admin := range admins {
		admin.SendPrivateMessage(bot, message)
	}
}

func HandleAdminCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	if err := state.DeleteStateEntry(update.FromChat().ID); err != nil {
		log.Errorf("Error clearing state: %s", err)
	}
	msg := tgbotapi.NewMessage(update.FromChat().ID, "Choose an option")
	adminKeyboard := state.CreateAdminKeyboard()
	msg.ReplyMarkup = adminKeyboard
	return msg
}
