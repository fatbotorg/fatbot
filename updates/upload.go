package updates

import (
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleProbationUploadMessage(update tgbotapi.Update, user users.User) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(update.FromChat().ID, "")
	completedUploads, err := user.LastTwoWorkoutsInPastHour()
	if err != nil {
		return msg, nil
	}
	if completedUploads {
		msg.Text = fmt.Sprintf("Welcome back %s!", user.GetName())
		msg.ReplyToMessageID = update.Message.MessageID
	}
	return msg, nil
}

func handleWorkoutUpload(update tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	var message string
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	user, err := users.GetUserById(update.SentFrom().ID)
	if err != nil {
		return msg, err
	}
	chatId := update.FromChat().ID
	if !user.IsInGroup(chatId) {
		if err := user.RegisterInGroup(chatId); err != nil {
			log.Errorf("Error registering user %s in new group %d", user.GetName(), chatId)
			sentry.CaptureException(err)
		}
	}
	lastWorkout, err := user.GetLastXWorkout(1, update.FromChat().ID)
	if err != nil {
		log.Warn(err)

	}
	if !lastWorkout.IsOlderThan(30) && !user.OnProbation {
		log.Warn("Workout not older than 30 minutes: %s", user.GetName())
		return msg, nil
	}
	if err := user.UpdateWorkout(update); err != nil {
		return msg, err
	}

	if user.OnProbation {
		chatId, err := user.GetSingleChatId()
		if err != nil {
			return msg, err
		}
		if err := user.FlagLastWorkout(chatId); err != nil {
			return msg, err
		}
		return handleProbationUploadMessage(update, user)
	}

	if lastWorkout.CreatedAt.IsZero() {
		message = fmt.Sprintf("%s nice work!\nThis is your first workout",
			user.GetName(),
		)
	} else {
		hours := time.Now().Sub(lastWorkout.CreatedAt).Hours()
		timeAgo := ""
		if int(hours/24) == 0 {
			timeAgo = fmt.Sprintf("%d hours ago", int(hours))
		} else {
			days := int(hours / 24)
			timeAgo = fmt.Sprintf("%d days and %d hours ago", days, int(hours)-days*24)
		}
		if err := user.LoadWorkoutsThisCycle(chatId); err != nil {
			return msg, err
		}

		message = fmt.Sprintf("%s %s\nYour last workout was on %s (%s)\nWorkouts this week: %d",
			user.GetName(),
			users.GetRandomWorkoutMessage(),
			lastWorkout.CreatedAt.Weekday(),
			timeAgo,
			len(user.Workouts),
		)
	}

	msg.Text = message
	msg.ReplyToMessageID = update.Message.MessageID
	return msg, nil
}
