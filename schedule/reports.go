package schedule

import (
	"fatbot/users"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	quickchartgo "github.com/henomis/quickchart-go"
)

type Leader struct {
	User     users.User
	Workouts int
}

func CreateChart(bot *tgbotapi.BotAPI) {
	groups := users.GetGroupsWithUsers()
	for _, group := range groups {
		if len(group.Users) == 0 {
			continue
		}
		fileName := fmt.Sprintf("%d.png", group.ChatID)
		usersWorkouts, leaders := collectUsersData(group)
		userNames := group.GetUserFixedNamesList()
		usersStringSlice := "'" + strings.Join(userNames, "', '") + "'"
		workoutsStringSlice := strings.Join(usersWorkouts, ", ")
		chartConfig := createChartConfig(usersStringSlice, workoutsStringSlice)
		qc := createQuickChart(chartConfig)
		file, err := os.Create(fileName)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		qc.Write(file)

		msg := tgbotapi.NewPhoto(group.ChatID, tgbotapi.FilePath(fileName))
		if len(leaders) == 1 {
			leader := leaders[0]
			msg.Caption = fmt.Sprintf(
				"Weekly summary:\n%s is the ⭐ with %d workouts!",
				leader.User.GetName(),
				leader.Workouts,
			)
			if err := leader.User.RegisterWeeklyLeaderEvent(); err != nil {
				log.Errorf("Error while registering weekly leader event: %s", err)
				sentry.CaptureException(err)
			}
		} else if len(leaders) > 1 {
			caption := fmt.Sprintf("Weekly summary:\n⭐ Leaders of the week with %d workouts:\n",
				leaders[0].Workouts)
			for _, leader := range leaders {
				caption = caption + leader.User.GetName() + " "
			}
			msg.Caption = caption
		}
		if _, err = bot.Send(msg); err != nil {
			log.Error(err)
			sentry.CaptureException(err)
		}
	}
}

func collectUsersData(group users.Group) (usersWorkouts []string, leaders []Leader) {
	leaders = append(leaders, Leader{
		User:     users.User{},
		Workouts: 0,
	})
	for _, user := range group.Users {
		userPastWeekWorkouts := user.GetPastWeekWorkouts(group.ChatID)
		usersWorkouts = append(usersWorkouts, fmt.Sprint(len(userPastWeekWorkouts)))
		if leaders[0].Workouts > len(userPastWeekWorkouts) {
			continue
		} else if leaders[0].Workouts < len(userPastWeekWorkouts) {
			leaders = []Leader{}
		}
		leaders = append(leaders, Leader{
			User:     user,
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
