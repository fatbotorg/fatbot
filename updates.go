package main

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleUpdates(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if update.Message == nil {
		if update.CallbackQuery != nil {
			return handleCallbacks(update, bot)
		}
		if update.InlineQuery != nil {
			//NOTE: "Ignoring inline"
			return nil
		}
		return fmt.Errorf("Cant read message, maybe I don't have access?")
	}
	if !update.Message.IsCommand() {
		if err := handleNonCommandUpdates(update, bot); err != nil {
			return err
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
	} else if update.FromChat().IsPrivate() {
		msg := tgbotapi.NewMessage(update.FromChat().ID, "")
		// TODO:
		// Think about doing this on an Inline message when tagged
		msg.Text = "Try /help"
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
