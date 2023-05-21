package admin

import (
	"fatbot/users"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleAdminDeleteLastCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	msg.Text = "Pick a user to delete last workout for"
	msg.ReplyMarkup = CreateUsersKeyboard(0)
	return msg
}

func HandleAdminRenameCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	if update.Message.CommandArguments() == "" {
		msg.Text = "Pick a user to rename"
		msg.ReplyMarkup = CreateUsersKeyboard(0)
	} else {
		args := update.Message.CommandArguments()
		argsSlice := strings.Split(args, " ")
		userId, _ := strconv.ParseInt(argsSlice[0], 10, 64)
		newName := argsSlice[1]
		user, err := users.GetUserById(userId)
		if err != nil {
			log.Error(err)
		}
		if err := user.Rename(newName); err != nil {
			log.Error(err)
			msg.Text = "There was an error with renaming"
		} else {
			msg.Text = fmt.Sprintf("Ok, renamed %d to %s", userId, newName)
		}
	}
	return msg
}

func HandleAdminPushWorkoutCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	if update.Message.CommandArguments() == "" {
		msg.Text = "Pick a user to change workout for"
		msg.ReplyMarkup = CreateUsersKeyboard(0)
	} else {
		args := update.Message.CommandArguments()
		argsSlice := strings.Split(args, " ")
		userId, _ := strconv.ParseInt(argsSlice[0], 10, 64)
		days, _ := strconv.ParseInt(argsSlice[1], 10, 64)
		user, err := users.GetUserById(userId)
		if err != nil {
			log.Error(err)
		}
		chatId, err := user.GetChatId()
		if err := user.PushWorkout(days, chatId); err != nil {
			log.Error(err)
			msg.Text = "There was an error with pushing workout"
		} else {
			msg.Text = fmt.Sprintf("Ok, pushed %d days for %s", days, user.GetName())
		}
	}
	return msg
}
