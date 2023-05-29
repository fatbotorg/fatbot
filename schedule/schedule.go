package schedule

import (
	"fatbot/reports"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-co-op/gocron"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func Init(bot *tgbotapi.BotAPI) {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		log.Fatalf("Bad timezone: %s", err)
	}
	scheduler := gocron.NewScheduler(loc)
	if _, err := scheduler.Every(1).Hours().Do(func() { scanUsers(bot) }); err != nil {
		log.Errorf("Strikes scheduler err: %s", err)
	}
	if _, err := scheduler.Every(1).Day().Saturday().At("18:00").Do(func() { reports.CreateChart(bot) }); err != nil {
		log.Errorf("Reports scheduler err: %s", err)
	}
	scheduler.StartAsync()
}
