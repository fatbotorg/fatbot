package schedule

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-co-op/gocron"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

func Init(bot *tgbotapi.BotAPI) {
	timezone := viper.GetString("timezone")
	reportHour := viper.GetInt64("report.hour")
	reportTime := fmt.Sprintf("%d:00", reportHour)
	location, err := time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("Bad timezone: %s", err)
	}
	scheduler := gocron.NewScheduler(location)
	if _, err := scheduler.Every(1).Hours().Do(func() { scanUsers(bot) }); err != nil {
		log.Errorf("Strikes scheduler err: %s", err)
	}
	if _, err := scheduler.Every(1).Day().Saturday().At(reportTime).Do(func() { CreateChart(bot) }); err != nil {
		log.Errorf("Reports scheduler err: %s", err)
	}
	scheduler.StartAsync()
}
