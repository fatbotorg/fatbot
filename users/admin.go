package users

import (
	"fatbot/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SendMessageToAdmins(bot *tgbotapi.BotAPI, message tgbotapi.MessageConfig) {
	admins := GetAdminUsers()
	for _, admin := range admins {
		admin.SendPrivateMessage(bot, message)
	}
}

func (user User) AddLocalAdmin(chatId int64) error {
	db := db.DBCon
	if group, err := GetGroup(chatId); err != nil {
		return err
	} else {
		if err := db.Model(&user).Association("GroupsAdmin").Append(&group); err != nil {
			return err
		}
	}
	return nil
}
