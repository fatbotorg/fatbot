package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handle_status_command(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	user := getUser(update.Message)

	if user.LastWorkout.IsZero() {
		log.Warn("no last workout")
		msg.Text = "I don't have your last workout yet."
	} else {
		lastworkout := user.LastWorkout
		currentTime := time.Now()
		diff := currentTime.Sub(lastworkout)
		msg.Text = fmt.Sprintf("%s, your last workout was on %s\nYou have %d days left.",
			user.Name,
			user.LastWorkout.Weekday(),
			int(5-diff.Hours()/24))
	}
	return msg
}

func handle_show_users_command(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	users := getUsers()
	message := ""
	for _, user := range users {
		var lastWorkout string
		if user.LastWorkout.IsZero() {
			lastWorkout = "no record"
		} else {
			lastWorkout = user.LastWorkout.Weekday().String()
		}
		message = message + fmt.Sprintf("%s [%s]", user.Username, lastWorkout) + "\n"
	}
	msg.Text = message
	return msg
}

func handle_workout_command(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

	recordUser := getUser(update.Message)
	var message string
	if recordUser.PhotoUpdate.IsZero() || time.Now().Sub(recordUser.PhotoUpdate).Hours() > 24 {
		message = fmt.Sprintf("%s, upload a photo and report again", recordUser.Username)
	} else {
		updateWorkout(update.Message.From.UserName, 0)
		if recordUser.LastWorkout.IsZero() {
			message = fmt.Sprintf("%s (%s) nice work!\nThis is your first workout",
				update.Message.From.FirstName,
				update.Message.From.UserName,
			)
		} else {
			hours := time.Now().Sub(recordUser.LastWorkout).Hours()
			timeAgo := ""
			if int(hours/24) == 0 {
				timeAgo = fmt.Sprintf("%d hours ago", int(hours))
			} else {
				days := int(hours / 24)
				timeAgo = fmt.Sprintf("%d days and %d hours ago", days, int(hours)-days*24)
			}
			message = fmt.Sprintf("%s (%s) nice work!\nYour last workout was on %s (%s)",
				update.Message.From.FirstName,
				update.Message.From.UserName,
				recordUser.LastWorkout.Weekday(),
				timeAgo,
			)
		}
	}
	msg.Text = message
	return msg
}

func handle_admin_delete_last_command(update tgbotapi.Update, bot *tgbotapi.BotAPI) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	config := &tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID:             update.FromChat().ID,
			SuperGroupUsername: "",
			UserID:             update.SentFrom().ID,
		},
	}
	member, err := bot.GetChatMember(*config)

	if err != nil {
		log.Error(err)
	} else {
		super := member.IsAdministrator() || member.IsCreator()
		if super {
			// change last workout for user
			username := strings.Trim(update.Message.CommandArguments(), "@")
			err := rollbackLastWorkout(username)
			if err != nil {
				log.Error(err)
			}
			msg.Text = fmt.Sprintf("%s last workout cancelled", username)
		}
	}
	return msg
}
