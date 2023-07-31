package users

import (
	"fatbot/db"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SendMessageToSuperAdmins(bot *tgbotapi.BotAPI, message tgbotapi.MessageConfig) {
	admins := GetSuperAdminUsers()
	for _, admin := range admins {
		admin.SendPrivateMessage(bot, message)
	}
}

func SendMessageToGroupAdmins(bot *tgbotapi.BotAPI, chatId int64, message tgbotapi.MessageConfig) {
	group, err := GetGroupWithAdmins(chatId)
	if err != nil {
		log.Error("can't get group admins", "err", err, "group", group.Title)
		return
	}
	if len(group.Admins) == 0 {
		log.Error("no admins for goup", "group", group.Title)
		return
	}
	for _, admin := range group.Admins {
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

func (user User) RemoveLocalAdmin(chatId int64) error {
	log.Debug("removing admin")
	db := db.DBCon
	if group, err := GetGroup(chatId); err != nil {
		return err
	} else {
		if err := db.Model(&user).Association("GroupsAdmin").Delete(&group); err != nil {
			return err
		}
	}
	return nil
}

func (user *User) loadManagedGroups() {
	db := db.DBCon
	if err := db.Preload("GroupsAdmin").Find(&user).Error; err != nil {
		log.Error(err)
	}
}

func (user User) IsLocalAdmin() (bool, error) {
	db := db.DBCon
	err := db.Preload("GroupsAdmin").Find(&user).Error
	if err != nil {
		return false, err
	}
	return len(user.GroupsAdmin) > 0, nil
}
