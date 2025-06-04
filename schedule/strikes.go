package schedule

import (
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func scanUsers(bot *tgbotapi.BotAPI) error {
	groups := users.GetGroupsWithUsers()
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
			if isNew, err := user.IsNew(group.ChatID); err != nil {
				log.Error(err)
				sentry.CaptureException(err)
			} else if isNew {
				log.Debug("new user not striking: %s", user.GetName())
				continue
			}
			lastWorkout, err := user.GetLastXWorkout(1, group.ChatID)
			if err != nil {
				log.Errorf("Err getting last workout for user %s: %s", user.GetName(), err)
				sentry.CaptureException(err)
			}

			_, daysDiff := users.IsLastWorkoutOverdue(lastWorkout.CreatedAt)
			if daysDiff == 4 && time.Now().Hour() == 19 {
				msg := tgbotapi.NewMessage(
					group.ChatID, fmt.Sprintf("[%s](tg://user?id=%d) you have one day left to workout",
						user.GetName(),
						user.TelegramUserID))
				msg.ParseMode = "MarkdownV2"
				bot.Send(msg)
				if err := user.RegisterLastDayNotificationEvent(); err != nil {
					log.Errorf("Error while registering ban event: %s", err)
					sentry.CaptureException(err)
				}
			} else if lastWorkoutOverdue, _ := users.
				IsLastWorkoutOverdue(lastWorkout.CreatedAt); lastWorkoutOverdue {
				if user.Immuned {
					user.SetImmunity(false)
					user.CreateDummyWorkout()
					bot.Send(
						tgbotapi.NewMessage(
							group.ChatID,
							fmt.Sprintf("Saved because of immunity: %s", user.GetName()),
						),
					)
					continue
				}
				if err := user.Ban(bot, group.ChatID); err != nil {
					err := fmt.Errorf("Issue banning %s from %d: %s", user.GetName(), group.ChatID, err)
					log.Error(err)
					sentry.CaptureException(err)
					continue
				}
			}
		}
	}
	return nil
}

func handleProbation(bot *tgbotapi.BotAPI, user users.User, group users.Group, totalDays float64) {
	lastWorkout, err := user.GetLastXWorkout(2, group.ChatID)
	if err != nil {
		log.Errorf("Err getting last 2 workout for user %s: %s", user.GetName(), err)
		sentry.CaptureException(err)
	}
	diffHours := int(totalDays*24 - time.Now().Sub(lastWorkout.CreatedAt).Hours())
	rejoinedLastHour := time.Now().Sub(user.UpdatedAt).Minutes() <= 60
	lastWorkoutOk := diffHours > 0
	if !lastWorkoutOk && !rejoinedLastHour {
		if errors := user.Ban(bot, group.ChatID); errors != nil {
			log.Errorf("Issue banning %s from %d: %s", user.GetName(), group.ChatID, errors)
			sentry.CaptureException(err)
		}
	} else if lastWorkoutOk {
		if err := user.UpdateOnProbation(false); err != nil {
			log.Errorf("Issue updating unprobation %s from %d: %s", user.GetName(), group.ChatID, err)
			sentry.CaptureException(err)
		}
	}
}
