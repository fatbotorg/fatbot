package main

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleUpdates(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if update.Message == nil {
		if update.CallbackQuery != nil {
			return handleCallbacks(update, bot)
		}
		return fmt.Errorf("Cant read message")
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
		msg, err := handleWorkoutCommand(update, bot)
		if err != nil {
			return fmt.Errorf("Error handling last workout: %s", err)
		}
		if msg.Text == "" {
			return nil
		}
		if _, err := bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func isAdminCommand(cmd string) bool {
	commandPrefix := strings.Split(cmd, "_")
	if len(commandPrefix) > 0 && commandPrefix[0] == "admin" {
		return true
	}
	return false
}

func handleCommandUpdate(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if !update.FromChat().IsPrivate() {
		return nil
	}
	if isAdminCommand(update.Message.Command()) {
		return handleAdminCommandUpdate(update, bot)
	}
	var msg tgbotapi.MessageConfig
	msg.Text = "Unknown command"
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
	case "help":
		msg.ChatID = update.FromChat().ID
		msg.Text = "/status"
	default:
		msg.ChatID = update.FromChat().ID
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func handleAdminCommandUpdate(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	var msg tgbotapi.MessageConfig
	msg.ChatID = update.FromChat().ID
	switch update.Message.Command() {
	case "admin_delete_last":
		msg = handleAdminDeleteLastCommand(update, bot)
	case "admin_rename":
		msg = handleAdminRenameCommand(update, bot)
	case "admin_help":
		msg.Text = "/admin_delete_last\n/admin_rename\n/admin_help"
	default:
		msg.Text = "Unknown command"
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}
