package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func tick(bot *tgbotapi.BotAPI, ticker *time.Ticker, done chan bool) {
	for {
		scanUsersForStrikes(bot)
		select {
		case <-done:
			return
		case t := <-ticker.C:
			log.Info("Tick at", t)
		}
	}
}

func scanUsersForStrikes(bot *tgbotapi.BotAPI) {
	db := getDB()
	var users []User
	db.Find(&users)
	for _, user := range users {
		diff := int(5 - time.Now().Sub(user.LastWorkout).Hours()/24)
		if diff == 1 && !user.WasNotified {
			log.Info("yep")
			msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("@%s you have one day left", user.Name))
			bot.Send(msg)
			db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("was_notified", true)
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
				updateUserInactive(user.TelegramUserID)
				msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("%s Wasn't active, so I kicked them", user.Name))
				bot.Send(msg)
			}
		}
	}
}
