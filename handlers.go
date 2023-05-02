package main

import (
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleStatusCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	user, err := users.GetUserFromMessage(update.Message)
	if err != nil {
		log.Error(err)
		return msg
	}
	lastWorkout, err := user.GetLastWorkout()
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
			user.GetName(),
			lastWorkout.CreatedAt.Weekday(),
			days,
			120-int(diff.Hours())-24*days-1,
		)
	}
	return msg
}

func handleShowUsersCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	// BUG: THIS GETS ALL USERS
	// use chat_id in the argument to get specific group
	users := users.GetUsers(0)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	message := ""
	var lastWorkoutStr string
	for _, user := range users {
		lastWorkout, err := user.GetLastWorkout()
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
		message = message + fmt.Sprintf("%s [%s]", user.GetName(), lastWorkoutStr) + "\n"
	}
	msg.Text = message
	return msg
}

func handleWorkoutCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI) (tgbotapi.MessageConfig, error) {
	var message string
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	user, err := users.GetUserFromMessage(update.Message)
	if err != nil {
		return msg, err
	}
	lastWorkout, err := user.GetLastWorkout()
	if err != nil {
		return msg, err
	}
	if !lastWorkout.IsOlderThan(30) {
		log.Warn("Workout not older than 30 minutes: %s", user.GetName())
		return msg, nil
	}
	if err := user.UpdateWorkout(update.Message.MessageID); err != nil {
		return msg, err
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
		message = fmt.Sprintf("%s nice work!\nYour last workout was on %s (%s)",
			user.GetName(),
			lastWorkout.CreatedAt.Weekday(),
			timeAgo,
		)
	}

	msg.Text = message
	msg.ReplyToMessageID = update.Message.MessageID
	return msg, nil
}

func handleJoinCommand(fatBotUpdate FatBotUpdate) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	if user, err := users.GetUserById(fatBotUpdate.Update.SentFrom().ID); err != nil {
		// TODO:
		// NOTE:
		from := fatBotUpdate.Update.Message.From
		adminMessage := tgbotapi.NewMessage(0,
			fmt.Sprintf(
				"User: %s %s %s is new and wants to join a group.",
				from.FirstName, from.LastName, from.UserName,
			),
		)
		sendMessageToAdmins(fatBotUpdate.Bot, adminMessage)
		msg.Text = "Hi! I have sent your request to the admin"
		return msg, err
	} else {
		msg.Text = fmt.Sprintf("Hi %s, I'm sending this for admin approval", user.GetName())
		adminMessage := tgbotapi.NewMessage(0, fmt.Sprintf("User %s wants to rejoin his group do you approve?", user.GetName()))
		var approvalKeyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprint(user.ID)),
				tgbotapi.NewInlineKeyboardButtonData("Decline", "false"),
			),
		)
		adminMessage.ReplyMarkup = approvalKeyboard
		sendMessageToAdmins(fatBotUpdate.Bot, adminMessage)
	}
	return
}
