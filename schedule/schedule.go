package schedule

import (
	"fatbot/users"
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
	if _, err := scheduler.Every(1).Wednesday().At(reportTime).Do(func() { ReportStandings(bot) }); err != nil {
		log.Errorf("Standings scheduler err: %s", err)
	}
	if _, err := scheduler.Every(1).MonthLastDay().Do(func() { nudgeBannedUsers(bot) }); err != nil {
		log.Errorf("Banned user nudge err: %s", err)
	}
	if _, err := scheduler.Every(1).MonthLastDay().At(reportTime).Do(func() { MonthlyReport(bot) }); err != nil {
		log.Errorf("Monthly report scheduler err: %s", err)
	}
	if _, err := scheduler.Every(1).Day().At("08:00").Do(func() {
		users.UpdateAllUserRanks()
	}); err != nil {
		log.Errorf("Rank updater scheduler err: %s", err)
	}

	scheduler.StartAsync()
}
