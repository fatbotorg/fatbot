package main

import (
	"fatbot/reports"
	"fatbot/users"
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func tickUsersScan(bot *tgbotapi.BotAPI, ticker *time.Ticker, done chan bool) {
	for {
		// TODO: REMOVE
		// reports.CreateChart(bot)
		// TODO: REMOVE

		if err := scanUsersForStrikes(bot); err != nil {
			log.Errorf("Scan users err: %s", err)
		}
		if time.Now().Weekday() == time.Saturday && time.Now().Hour() == 18 {
			reports.CreateChart(bot)
		}
		select {
		case <-done:
			return
		case t := <-ticker.C:
			log.Info("Tick at", t)
		}
	}
}

func scanUsersForStrikes(bot *tgbotapi.BotAPI) error {
	db := getDB()
	var users []users.User
	db.Find(&users)
	for _, user := range users {
		lastWorkout, err := user.GetLastWorkout()
		if err != nil {
			log.Errorf("Err getting last workout for user %s: %s", user.GetName(), err)
			continue
		}
		diff := int(math.Ceil(5 - time.Now().Sub(lastWorkout.CreatedAt).Hours()/24))
		if diff == 1 && !user.WasNotified {
			msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("[%s](tg://user?id=%d) you have 24 hours left",
				user.GetName(),
				user.TelegramUserID))
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			if err != user.UpdateWasNotified(true) {
				return fmt.Errorf("Error with marking user %s as notified: %s", user.GetName(), err)
			}
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
				return err
			} else if user.IsActive {
				user.UpdateInactive()
				user.UpdateWasNotified(false)
				msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("It's been 5 full days since %s worked out.\nI kicked them", user.GetName()))
				bot.Send(msg)
			}
		}
	}
	return nil
}
