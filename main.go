package main

import (
	"fatbot/db"
	"fatbot/schedule"
	"fatbot/updates"
	"fatbot/users"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

func initViper() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("fatal error config file: %s", err)
	}
	viper.AutomaticEnv()
}

func main() {
	var bot *tgbotapi.BotAPI
	var err error
	var updatesChannel tgbotapi.UpdatesChannel
	// Init Config
	initViper()
	// Init DB
	db.DBCon = db.GetDB()
	log.SetLevel(log.DebugLevel)
	if os.Getenv("ENVIRONMENT") != "production" {
		log.SetLevel(log.DebugLevel)
	} else {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              os.Getenv("SENTRY_DSN"),
			TracesSampleRate: 1.0,
		})
		if err != nil {
			log.Fatalf("sentry.Init: %s", err)
		}
		defer sentry.Flush(2 * time.Second)
	}
	if err := users.InitDB(); err != nil {
		log.Fatal(err)
	}

	if bot, err = tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN")); err != nil {
		log.Fatalf("Issue with token: %s", err)
	} else {
		schedule.Init(bot)
		bot.Debug = false
		log.Infof("Authorized on account %s", bot.Self.UserName)
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updatesChannel = bot.GetUpdatesChan(u)
	}
	fatBotUpdate := updates.FatBotUpdate{Bot: bot}
	for update := range updatesChannel {
		fatBotUpdate.Update = update
		if err := updates.HandleUpdates(fatBotUpdate); err != nil {
			log.Error(err)
			sentry.CaptureException(err)
		}
	}
}
