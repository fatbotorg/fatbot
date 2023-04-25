package main

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	var bot *tgbotapi.BotAPI
	var err error
	var updates tgbotapi.UpdatesChannel
	if err := initDB(); err != nil {
		log.Fatal(err)
	}
	if bot, err = tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN")); err != nil {
		log.Fatal(err)
	} else {
		go tick(bot, time.NewTicker(24*time.Hour), make(chan bool))
		bot.Debug = false
		log.Infof("Authorized on account %s", bot.Self.UserName)
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates = bot.GetUpdatesChan(u)
	}

	for update := range updates {
		if update.Message == nil { // ignore any non-Message updates
			continue
		}
		if !update.Message.IsCommand() { // ignore any non-command Messages
			if len(update.Message.Photo) > 0 {
				updateUserImage(update.Message.From.UserName)
			}
			continue
		}

		var msg tgbotapi.MessageConfig
		switch update.Message.Command() {
		case "status":
			msg = handle_status_command(update)
		case "show_users":
			msg = handle_show_users_command(update)
		case "workout":
			msg = handle_workout_command(update)
		case "admin_delete_last":
			msg = handle_admin_delete_last_command(update, bot)
		default:
			msg.Text = "Unknown command"
		}
		if _, err := bot.Send(msg); err != nil {
			log.Error(err)
		}
	}
}
