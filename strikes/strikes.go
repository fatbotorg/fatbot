package strikes

import (
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ScanUsers(bot *tgbotapi.BotAPI) error {
	users := users.GetUsers(0)
	const totalDays = 5.0
	for _, user := range users {
		// if user.ID == 18 {
		// 	user.UnBan(bot)
		// 	continue
		// }
		if !user.Active {
			continue
		}
		if user.OnProbation {
			handleProbation(bot, user, totalDays)
			continue
		}
		lastWorkout, err := user.GetLastXWorkout(1)
		if err != nil {
			log.Errorf("Err getting last workout for user %s: %s", user.GetName(), err)
		}
		diffHours := int(totalDays*24 - time.Now().Sub(lastWorkout.CreatedAt).Hours())
		if diffHours == 23 {
			msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("[%s](tg://user?id=%d) you have 24 hours left",
				user.GetName(),
				user.TelegramUserID))
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			if err := user.RegisterLastDayNotificationEvent(); err != nil {
				log.Errorf("Error while registering ban event: %s", err)
			}
		} else if diffHours <= 0 {
			if err := user.Ban(bot); err != nil {
				log.Errorf("Issue banning %s from %d: %s", user.GetName(), user.ChatID, err)
				continue
			}
		}
	}
	return nil
}

func handleProbation(bot *tgbotapi.BotAPI, user users.User, totalDays float64) {
	log.Debug("Probation", "user.OnProbation", user.OnProbation)
	lastWorkout, err := user.GetLastXWorkout(2)
	if err != nil {
		log.Errorf("Err getting last 2 workout for user %s: %s", user.GetName(), err)
	}
	diffHours := int(totalDays*24 - time.Now().Sub(lastWorkout.CreatedAt).Hours())
	log.Debug("Probation", "diffHours", diffHours)
	rejoinedLastHour := time.Now().Sub(user.UpdatedAt).Minutes() <= 60
	lastTwoWorkoutsOk := diffHours > 0
	if !lastTwoWorkoutsOk && !rejoinedLastHour {
		if errors := user.Ban(bot); errors != nil {
			log.Errorf("Issue banning %s from %d: %s", user.GetName(), user.ChatID, errors)
		}
	} else if lastTwoWorkoutsOk {
		log.Debug("Probation", "lastTwoWorkoutsOk", lastTwoWorkoutsOk)
		if err := user.UpdateOnProbation(false); err != nil {
			log.Errorf("Issue updating unprobation %s from %d: %s", user.GetName(), user.ChatID, err)
		}
	}
}
