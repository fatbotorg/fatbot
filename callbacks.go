package main

import (
	"fatbot/users"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleCallbacks(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	switch update.CallbackQuery.Message.Text {
	case "Pick a user to delete last workout for":
		callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
		if _, err := bot.Request(callback); err != nil {
			panic(err)
		}
		userId, _ := strconv.ParseInt(update.CallbackQuery.Data, 10, 64)
		user, err := users.GetUserById(userId)
		if err != nil {
			return err
		}
		if newLastWorkout, err := user.RollbackLastWorkout(); err != nil {
			return err
		} else {
			message := fmt.Sprintf("Deleted last workout for user %s\nRolledback to: %s",
				user.GetName(), newLastWorkout.CreatedAt.Format("2006-01-02 15:04:05"))
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, message)
			if _, err := bot.Send(msg); err != nil {
				return err
			}
			user.SendPrivateMessage(bot, fmt.Sprintf("Your last workout was cancelled by the admin.\nUpdated workout: %s", newLastWorkout.CreatedAt))
		}
	case "Pick a user to rename":
		callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
		if _, err := bot.Request(callback); err != nil {
			panic(err)
		}
		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
		msg.Text = fmt.Sprintf("/admin_rename %s newname", update.CallbackData())
		if _, err := bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}
