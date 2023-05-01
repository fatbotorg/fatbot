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

// NOTE:
// This needs to be its own module and it needs rafactoring
// Mainly the strikes scan

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
		totalDays := 5.0
		diffHours := int(totalDays*24 - time.Now().Sub(lastWorkout.CreatedAt).Hours())
		if diffHours == 23 {
			msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("[%s](tg://user?id=%d) you have 24 hours left",
				user.GetName(),
				user.TelegramUserID))
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			if err != user.IncrementNotificationCount() {
				return fmt.Errorf("Error with bumping user %s notifications: %s",
					user.GetName(), err)
			}
		} else if diffHours <= 0 {
			if err := user.Ban(bot); err != nil {
				return err
			}
		}
	}
	return nil
}
