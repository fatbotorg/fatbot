package main

import (
	"fatbot/users"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func createUsersKeyboard() tgbotapi.InlineKeyboardMarkup {
	users := users.GetUsers()
	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, user := range users {
		userLabel := fmt.Sprintf("%s", user.GetName())
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(userLabel, fmt.Sprint(user.TelegramUserID)))
		if len(row) == 3 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 && len(row) < 3 {
		rows = append(rows, row)
	}
	var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}

func handleAdminDeleteLastCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	msg.Text = "Pick a user to delete last workout for"
	msg.ReplyMarkup = createUsersKeyboard()
	return msg
}

func handleAdminRenameCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	if update.Message.CommandArguments() == "" {
		msg.Text = "Pick a user to rename"
		msg.ReplyMarkup = createUsersKeyboard()
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
