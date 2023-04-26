package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleStatusCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	user := getUser(update.Message)

	lastWorkout, err := getLastWorkout(user.TelegramUserID)
	if err != nil {
		log.Errorf("Err getting last workout: %s", err)
		return msg
	}
	if lastWorkout.CreatedAt.IsZero() {
		log.Warn("no last workout")
		msg.Text = "I don't have your last workout yet."
	} else {
		currentTime := time.Now()
		diff := currentTime.Sub(lastWorkout.CreatedAt)
		days := int(5 - diff.Hours()/24)
		msg.Text = fmt.Sprintf("%s, your last workout was on %s\nYou have %d days and %d hours left.",
			user.Name,
			lastWorkout.CreatedAt.Weekday(),
			days,
			120-int(diff.Hours())-24*days-1,
		)
	}
	return msg
}

func handleShowUsersCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	users := getUsers()
	message := ""
	var lastWorkoutStr string
	for _, user := range users {
		lastWorkout, err := getLastWorkout(user.TelegramUserID)
		if err != nil {
			log.Errorf("Err getting last workout: %s", err)
			return msg
		}
		if lastWorkout.CreatedAt.IsZero() {
			lastWorkoutStr = "no record"
		} else {
			hour, min, _ := lastWorkout.CreatedAt.Clock()
			lastWorkoutStr = fmt.Sprintf("%s, %d:%d", lastWorkout.CreatedAt.Weekday().String(), hour, min)
		}
		message = message + fmt.Sprintf("%s [%s]", user.Name, lastWorkoutStr) + "\n"
	}
	msg.Text = message
	return msg
}

func handleWorkoutCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	user := getUser(update.Message)
	lastWorkout, err := getLastWorkout(user.TelegramUserID)
	if err != nil {
		log.Errorf("Err getting last workout: %s", err)
		return msg
	}
	var message string
	updateWorkout(update.Message.From.ID, update.Message.MessageID)
	if lastWorkout.CreatedAt.IsZero() {
		message = fmt.Sprintf("%s nice work!\nThis is your first workout",
			update.Message.From.FirstName,
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
		message = fmt.Sprintf("%s nice work!\nYour last workout was on %s (%s)",
			update.Message.From.FirstName,
			lastWorkout.CreatedAt.Weekday(),
			timeAgo,
		)
	}

	msg.Text = message
	msg.ReplyToMessageID = update.Message.MessageID
	return msg
}
