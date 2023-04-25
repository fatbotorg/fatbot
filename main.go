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
		log.Fatalf("Issue with token: %s", err)
	} else {
		go tick(bot, time.NewTicker(24*time.Hour), make(chan bool))
		bot.Debug = false
		log.Infof("Authorized on account %s", bot.Self.UserName)
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates = bot.GetUpdatesChan(u)
	}

	for update := range updates {
		if err := handle_update(update, bot); err != nil {
			log.Error(err)
		}
	}
}
