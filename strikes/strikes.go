package strikes

import (
	"fatbot/migrations"
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ScanUsers(bot *tgbotapi.BotAPI) error {
	log.Info("migrations running")
	migrations.CreateGroupsMigration()
	migrations.WorkoutsToGroupsMigration()
	groups := users.GetGroupsWithUsers()
	users := users.GetUsers(-1)
	for _, user := range users {
		migrations.ChatIdToGroupsMigration(&user)
	}
	const totalDays = 5.0
	for _, group := range groups {
		for _, user := range group.Users {
			if !user.Active {
				continue
			}
			if user.OnProbation {
				handleProbation(bot, user, group, totalDays)
				continue
			}
			lastWorkout, err := user.GetLastXWorkout(1, group.ChatID)
			if err != nil {
				log.Errorf("Err getting last workout for user %s: %s", user.GetName(), err)
			}
			diffHours := int(totalDays*24 - time.Now().Sub(lastWorkout.CreatedAt).Hours())
			if diffHours == 23 {
				msg := tgbotapi.NewMessage(group.ChatID, fmt.Sprintf("[%s](tg://user?id=%d) you have 24 hours left",
					user.GetName(),
					user.TelegramUserID))
				msg.ParseMode = "MarkdownV2"
				bot.Send(msg)
				if err := user.RegisterLastDayNotificationEvent(); err != nil {
					log.Errorf("Error while registering ban event: %s", err)
				}
			} else if diffHours <= 0 {
				if err := user.Ban(bot, group.ChatID); err != nil {
					log.Errorf("Issue banning %s from %d: %s", user.GetName(), group.ChatID, err)
					continue
				}
			}
		}
	}
	return nil
}

func handleProbation(bot *tgbotapi.BotAPI, user users.User, group users.Group, totalDays float64) {
	log.Debug("Probation", "user.OnProbation", user.OnProbation)
	lastWorkout, err := user.GetLastXWorkout(2, group.ChatID)
	if err != nil {
		log.Errorf("Err getting last 2 workout for user %s: %s", user.GetName(), err)
	}
	diffHours := int(totalDays*24 - time.Now().Sub(lastWorkout.CreatedAt).Hours())
	log.Debug("Probation", "diffHours", diffHours)
	rejoinedLastHour := time.Now().Sub(user.UpdatedAt).Minutes() <= 60
	lastTwoWorkoutsOk := diffHours > 0
	if !lastTwoWorkoutsOk && !rejoinedLastHour {
		if errors := user.Ban(bot, group.ChatID); errors != nil {
			log.Errorf("Issue banning %s from %d: %s", user.GetName(), group.ChatID, errors)
		}
	} else if lastTwoWorkoutsOk {
		log.Debug("Probation", "lastTwoWorkoutsOk", lastTwoWorkoutsOk)
		if err := user.UpdateOnProbation(false); err != nil {
			log.Errorf("Issue updating unprobation %s from %d: %s", user.GetName(), group.ChatID, err)
		}
	}
}
