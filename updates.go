package main

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleUpdates(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	if update.Message == nil { // ignore any non-Message updates
		if update.CallbackQuery != nil {
			switch update.CallbackQuery.Message.Text {
			case "Pick a user":
				callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
				if _, err := bot.Request(callback); err != nil {
					panic(err)
				}
				userId, _ := strconv.ParseInt(update.CallbackQuery.Data, 10, 64)
				if newLastWorkout, err := rollbackLastWorkout(userId); err != nil {
					return err
				} else {
					message := fmt.Sprintf("Deleted last workout for user %d\nRolledback to: %s", userId, newLastWorkout.CreatedAt.Format("2006-01-02 15:04:05"))
					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, message)
					if _, err := bot.Send(msg); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if !update.Message.IsCommand() { // ignore any non-command Messages
		if len(update.Message.Photo) > 0 || update.Message.Video != nil {
			if lastWorkout, err := getLastWorkout(update.Message.From.ID); err != nil {
				return err
			} else if !isToday(lastWorkout.CreatedAt) {
				msg := handleWorkoutCommand(update, bot)
				if _, err := bot.Send(msg); err != nil {
					return err
				}
			}
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
	default:
		msg.Text = "Unknown command"
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}
