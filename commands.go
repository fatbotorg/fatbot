package main

import (
	"fatbot/reports"
	"fatbot/users"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleCommandUpdate(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	if !update.FromChat().IsPrivate() {
		return nil
	}
	if isAdminCommand(update.Message.Command()) {
		return handleAdminCommandUpdate(update, bot)
	}
	if !update.FromChat().IsPrivate() {
		// NOTE: not allowing non-private commands ATM
		return nil
	}
	var err error
	var msg tgbotapi.MessageConfig
	msg.Text = "Unknown command"
	switch update.Message.Command() {
	case "join":
		msg, err = handleJoinCommand(fatBotUpdate)
		if err != nil {
			return err
		}
	case "status":
		msg = handleStatusCommand(update)
	case "help":
		msg.ChatID = update.FromChat().ID
		msg.Text = "/status\n/rejoin"
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

	user, err := users.GetUserById(update.Message.From.ID)
	if err != nil {
		return err
	}
	if !user.IsAdmin {
		return nil
	}
	switch update.Message.Command() {
	case "admin_show_users":
		//TODO:
		// handlr pergroup
		if update.FromChat().IsPrivate() {
			msg = handleShowUsersCommand(update)
		}
	case "admin_delete_last":
		msg = handleAdminDeleteLastCommand(update, bot)
	case "admin_rename":
		msg = handleAdminRenameCommand(update, bot)
	case "admin_help":
		msg.Text = "/admin_delete_last\n/admin_rename\n/admin_help\n/admin_show_users"
	case "admin_send_report":
		reports.CreateChart(bot)
	default:
		msg.Text = "Unknown command"
	}
	if msg.Text == "" {
		return nil
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}
