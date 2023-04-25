package main

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/gorm"
)

func scanUsers(bot *tgbotapi.BotAPI) {
	db, err := gorm.Open(sqlite.Open(getDB()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	var users []User
	db.Find(&users)
	for _, user := range users {
		diff := int(5 - time.Now().Sub(user.LastWorkout).Hours()/24)
		if diff == 1 && !user.WasNotified {
			log.Info("yep")
			msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("@%s you have one day left", user.Username))
			bot.Send(msg)
			db.Model(&user).Where("username = ?", user.Username).Update("was_notified", true)
		} else if diff == 0 {
			banChatMemberConfig := tgbotapi.BanChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{
					ChatID:             user.ChatID,
					SuperGroupUsername: "shotershmenimbot",
					ChannelUsername:    "",
					UserID:             user.TelegramUserID,
				},
				UntilDate:      0,
				RevokeMessages: false,
			}
			_, err := bot.Request(banChatMemberConfig)
			if err != nil {
				log.Error(err)
			} else if user.IsActive {
				updateUserInactive(user.Username)
				msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("%s Wasn't active, so I kicked them", user.Name))
				bot.Send(msg)
			}
		}
	}
}
