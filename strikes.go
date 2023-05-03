package main

import (
	"fatbot/reports"
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// NOTE:
// This needs to be its own module and it needs rafactoring
// Mainly the strikes scan

func tickUsersScan(bot *tgbotapi.BotAPI, ticker *time.Ticker, done chan bool) {
	for {
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
	const totalDays = 5.0
	for _, user := range users {
		if user.OnProbation {
			log.Debug("Probation", "user.OnProbation", user.OnProbation)
			if lastWorkout, err := user.GetLastXWorkout(2); err != nil {
				log.Errorf("Err getting last 2 workout for user %s: %s", user.GetName(), err)
			} else {
				diffHours := int(totalDays*24 - time.Now().Sub(lastWorkout.CreatedAt).Hours())
				log.Debug("Probation", "diffHours", diffHours)
				rejoinedLastHour := time.Now().Sub(user.UpdatedAt).Minutes() <= 60
				lastTwoWorkoutsOk := diffHours > 0
				if !lastTwoWorkoutsOk && !rejoinedLastHour {
					if err := user.Ban(bot); err != nil {
						log.Errorf("Issue banning %s from %d: %s", user.GetName(), user.ChatID, err)
					}
				} else if lastTwoWorkoutsOk {
					log.Debug("Probation", "lastTwoWorkoutsOk", lastTwoWorkoutsOk)
					if err := user.UpdateOnProbation(false); err != nil {
						log.Errorf("Issue updating unprobation %s from %d: %s", user.GetName(), user.ChatID, err)
					}
				}
			}
			continue
		}
		lastWorkout, err := user.GetLastXWorkout(1)
		if err != nil {
			log.Errorf("Err getting last workout for user %s: %s", user.GetName(), err)
			continue
		}
		diffHours := int(totalDays*24 - time.Now().Sub(lastWorkout.CreatedAt).Hours())
		if diffHours == 23 {
			msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("[%s](tg://user?id=%d) you have 24 hours left",
				user.GetName(),
				user.TelegramUserID))
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			// TODO:
			// Add an event for the user
			//
			// if err != user.IncrementNotificationCount() {
			// 	return fmt.Errorf("Error with bumping user %s notifications: %s",
			// 		user.GetName(), err)
			// }
		} else if diffHours <= 0 {
			if err := user.Ban(bot); err != nil {
				log.Errorf("Issue banning %s from %d: %s", user.GetName(), user.ChatID, err)
				continue
			}
		}
	}
	return nil
}
