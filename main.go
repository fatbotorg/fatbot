package main

import (
	"fatbot/db"
	"fatbot/schedule"
	"fatbot/updates"
	"fatbot/users"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

func initViper() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("fatal error config file: %s!", err)
	}
	viper.AutomaticEnv()
}

// setupBotCommands sets up the bot commands menu that appears next to the message line
func setupBotCommands(bot *tgbotapi.BotAPI) {
	// Define commands for regular users in private chats
	userCommands := []tgbotapi.BotCommand{
		{
			Command:     "start",
			Description: "Start using the bot",
		},
		{
			Command:     "join",
			Description: "Join a group",
		},
		{
			Command:     "status",
			Description: "Check your workout status",
		},
		{
			Command:     "stats",
			Description: "View workout statistics",
		},
		{
			Command:     "whoop",
			Description: "Connect Whoop Account",
		},
		{
			Command:     "help",
			Description: "Show help information",
		},
		{
			Command:     "admin",
			Description: "Admin only: Access administrative functions",
		},
	}

	// Setup commands for all private chats
	privateScope := tgbotapi.NewBotCommandScopeAllPrivateChats()
	privateChatCommands := tgbotapi.NewSetMyCommandsWithScope(privateScope, userCommands...)

	if _, err := bot.Request(privateChatCommands); err != nil {
		log.Error("Failed to set up private chat commands menu", "error", err)
		sentry.CaptureException(err)
	}

	log.Info("Bot commands menu has been set up successfully")
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

		go func() {
			http.HandleFunc("/whoop-callback", updates.HandleWhoopCallback)
			port := os.Getenv("PORT")
			if port == "" {
				port = "8080"
			}
			log.Infof("Starting HTTP server on port %s", port)
			if err := http.ListenAndServe(":"+port, nil); err != nil {
				log.Errorf("HTTP server failed: %s", err)
			}
		}()

		// Set up the bot commands menu
		setupBotCommands(bot)

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
