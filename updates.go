package main

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleUpdates(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if update.Message == nil { // ignore any non-Message updates
		return nil
	}
	if !update.Message.IsCommand() { // ignore any non-command Messages
		if len(update.Message.Photo) > 0 || update.Message.Video != nil {
			updateUserImage(update.Message.From.ID)
		}
		return nil
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

func handleCommandUpdate(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	var msg tgbotapi.MessageConfig
	switch update.Message.Command() {
	case "status":
		if update.FromChat().IsPrivate() {
			msg = handleStatusCommand(update)
		}
	case "show_users":
		if update.FromChat().IsPrivate() {
			msg = handleShowUsersCommand(update)
		}
	case "workout":
		if update.FromChat().IsPrivate() {
			return nil
		}
		msg = handleWorkoutCommand(update, bot)
	// case "admin_delete_last":
	// 	msg = handle_admin_delete_last_command(update, bot)
	default:
		msg.Text = "Unknown command"
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}
