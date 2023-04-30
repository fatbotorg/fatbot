package reports

import (
	"fatbot/accounts"
	"fatbot/users"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	quickchartgo "github.com/henomis/quickchart-go"
)

func CreateChart(bot *tgbotapi.BotAPI) {
	//TODO:
	// get leader
	var usersNames []string
	var usersWorkouts []string
	var chatId int64
	// var leader map[string]int
	accounts := accounts.GetAccounts()
	for _, account := range accounts {

		// TODO: TEMP this is still in beta
		if account.ChatID != -1001899294753 {
			continue
		}
		// TODO: TEMP

		users := users.GetUsers()
		for _, user := range users {
			if user.ChatID != account.ChatID {
				continue
			}
			usersNames = append(usersNames, user.GetName())
			usersWorkouts = append(usersWorkouts, fmt.Sprint(len(user.GetWorkouts())))
			if chatId == 0 {
				chatId = user.ChatID
			}
		}
		usersStringSlice := "'" + strings.Join(usersNames, "', '") + "'"
		workoutsStringSlice := strings.Join(usersWorkouts, ", ")
		chartConfig := fmt.Sprintf(`{
		type: 'bar',
		data: {
			labels: [%s],
			datasets: [{
			label: 'Workouts',
			data: [%s]
			}]
		}
	}`, usersStringSlice, workoutsStringSlice)
		qc := quickchartgo.New()
		qc.Config = chartConfig
		qc.Width = 500
		qc.Height = 300
		qc.Version = "2.9.4"
		qc.BackgroundColor = "white"
		file, err := os.Create("output.png")
		if err != nil {
			panic(err)
		}

		defer file.Close()
		qc.Write(file)

		msg := tgbotapi.NewPhoto(chatId, tgbotapi.FilePath("output.png"))
		msg.Caption = "Weekly summary"
		if _, err = bot.Send(msg); err != nil {
			log.Error(err)
		}
	}
}
