package main

import (
	"fatbot/users"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleUpdates(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	if update.SentFrom() == nil {
		return fmt.Errorf("can't handle update with no SentFrom details...")
	}
	if !users.IsApprovedChatID(update.FromChat().ID) && !update.FromChat().IsPrivate() {
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("Group %s not activated, send this to the admin: `%d`", update.Message.Chat.Title, update.FromChat().ID),
		))
		sentry.CaptureMessage(fmt.Sprintf("non activated group: %d, title: %s", update.FromChat().ID, update.FromChat().Title))
		return nil
	}
	if users.BlackListed(update.SentFrom().ID) {
		log.Debug("Blocked", "id", update.SentFrom().ID)
		return nil
	}
	if fatBotUpdate.Update.Message == nil {
		if fatBotUpdate.Update.CallbackQuery != nil {
			return handleCallbacks(fatBotUpdate)
		}
		if fatBotUpdate.Update.InlineQuery != nil {
			//NOTE: "Ignoring inline"
			return nil
		}
		return fmt.Errorf("Cant read message, maybe I don't have access?")
	}
	if !fatBotUpdate.Update.Message.IsCommand() {
		if err := handleNonCommandUpdates(fatBotUpdate); err != nil {
			return err
		}
		return nil
	}

	if err := handleCommandUpdate(fatBotUpdate); err != nil {
		return err
	}
	return nil
}

func handleNonCommandUpdates(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	if len(update.Message.Photo) > 0 || update.Message.Video != nil {
		if update.FromChat().IsPrivate() {
			return nil
		}
		msg, err := handleWorkoutUpload(update)
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
		if err := handleStatefulCallback(fatBotUpdate); err == nil {
			return err
		}
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
