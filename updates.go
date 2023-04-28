package main

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleUpdates(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if update.Message == nil && update.CallbackQuery != nil {
		return handleCallbacks(update, bot)
	}
	if !update.Message.IsCommand() {
		if err := handleNonCommandUpdates(update, bot); err != nil {
			return err
		}
	}

	if !isApprovedChatID(update.FromChat().ID) && !update.FromChat().IsPrivate() {
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("Group %s not activated, send this to the admin: `%d`", update.Message.Chat.Title, update.FromChat().ID),
		))
		return nil
	}
	if err := handleCommandUpdate(update, bot); err != nil {
		return err
	}
	return nil
}

func handleNonCommandUpdates(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if len(update.Message.Photo) > 0 || update.Message.Video != nil {
		if update.FromChat().IsPrivate() {
			return nil
		}
		msg := handleWorkoutCommand(update, bot)
		if _, err := bot.Send(msg); err != nil {
			return err
		}
		// NOTE:
		// Restore when you decide whether only one workout per day / one photo a day is the right setup
		// if lastWorkout, err := getLastWorkout(update.Message.From.ID); err != nil {
		// 	return err
		// } else if !isToday(lastWorkout.CreatedAt) {
		// 	msg := handleWorkoutCommand(update, bot)
		// 	if _, err := bot.Send(msg); err != nil {
		// 		return err
		// 	}
		// }
	}
	return nil
}

func isToday(when time.Time) bool {
	return time.Now().Sub(when).Hours() < 24
}

func isAdminCommand(cmd string) bool {
	commandPrefix := strings.Split(cmd, "_")
	if len(commandPrefix) > 0 && commandPrefix[0] == "admin" {
		return true
	}
	return false
}

func handleCommandUpdate(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if isAdminCommand(update.Message.Command()) {
		return handleAdminCommandUpdate(update, bot)
	}
	var msg tgbotapi.MessageConfig
	switch update.Message.Command() {
	case "status":
		if update.FromChat().IsPrivate() {
			msg = handleStatusCommand(update)
		}
	case "show_users":
		//TODO:
		// handlr pergroup
		if update.FromChat().IsPrivate() {
			msg = handleShowUsersCommand(update)
		}
	default:
		msg.Text = "Unknown command"
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func handleAdminCommandUpdate(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if !update.FromChat().IsPrivate() {
		return nil
	}
	var msg tgbotapi.MessageConfig
	switch update.Message.Command() {
	case "admin_delete_last":
		msg = handleAdminDeleteLastCommand(update, bot)
	case "admin_rename":
		msg = handleAdminRenameCommand(update, bot)
	case "admin_help":
		msg.ChatID = update.FromChat().ID
		msg.Text = "/admin_delete_last\n/admin_rename\n/admin_help"
	default:
		msg.ChatID = update.FromChat().ID
		msg.Text = "Unknown command"
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}
