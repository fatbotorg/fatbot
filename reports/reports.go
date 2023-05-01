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

type Leader struct {
	Name     string
	Workouts int
}

func CreateChart(bot *tgbotapi.BotAPI) {
	fileName := "output.png"
	accounts := accounts.GetAccounts()
	for _, account := range accounts {
		usersNames, usersWorkouts, leaders := collectUsersData(account.ChatID)
		if len(usersNames) == 0 {
			continue
		}
		usersStringSlice := "'" + strings.Join(usersNames, "', '") + "'"
		workoutsStringSlice := strings.Join(usersWorkouts, ", ")
		chartConfig := createChartConfig(usersStringSlice, workoutsStringSlice)
		qc := createQuickChart(chartConfig)
		file, err := os.Create(fileName)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		qc.Write(file)

		// TODO: chagne number back to account.chatId
		msg := tgbotapi.NewPhoto(9658139, tgbotapi.FilePath(fileName))
		// TODO:
		// Count the leaders!
		if len(leaders) == 1 {
			leader := leaders[0]
			msg.Caption = fmt.Sprintf(
				"Weekly summary:\n%s is the ⭐ with %d workouts!",
				leader.Name,
				leader.Workouts,
			)
		} else if len(leaders) > 1 {
			caption := fmt.Sprintf("Weekly summary:\n⭐ Leaders of the week with %d workouts:\n",
				leaders[0].Workouts)
			for _, leader := range leaders {
				caption = caption + leader.Name + " "
			}
			msg.Caption = caption
		}
		if _, err = bot.Send(msg); err != nil {
			log.Error(err)
		}
	}
}

func collectUsersData(accountChatId int64) (usersNames, usersWorkouts []string, leaders []Leader) {
	users := users.GetUsers()
	leaders = append(leaders, Leader{
		Name:     "",
		Workouts: 0,
	})
	for _, user := range users {
		if user.ChatID != accountChatId {
			continue
		}
		usersNames = append(usersNames, user.GetName())
		userPastWeekWorkouts := user.GetPastWeekWorkouts()
		usersWorkouts = append(usersWorkouts, fmt.Sprint(len(userPastWeekWorkouts)))
		if leaders[0].Workouts > len(userPastWeekWorkouts) {
			continue
		} else if leaders[0].Workouts < len(userPastWeekWorkouts) {
			leaders = []Leader{}
		}
		leaders = append(leaders, Leader{
			Name:     user.GetName(),
			Workouts: len(userPastWeekWorkouts),
		})
	}
	return
}

func createChartConfig(usersStringSlice, workoutsStringSlice string) string {
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
	return chartConfig
}

func createQuickChart(chartConfig string) *quickchartgo.Chart {
	qc := quickchartgo.New()
	qc.Config = chartConfig
	qc.Width = 500
	qc.Height = 300
	qc.Version = "2.9.4"
	qc.BackgroundColor = "white"
	return qc
}
