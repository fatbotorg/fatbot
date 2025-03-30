package schedule

import (
	"fatbot/users"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	quickchartgo "github.com/henomis/quickchart-go"
	"github.com/spf13/viper"
)

type Leader struct {
	User     users.User
	Workouts int
}

func nudgeBannedUsers(bot *tgbotapi.BotAPI) {
	inactiveUsers := users.GetInactiveUsers(0)
	for _, user := range inactiveUsers {
		lastBanDate, err := user.GetLastBanDate()
		if err != nil {
			log.Error(err)
			continue
		}
		timeSinceBan := int(time.Now().Sub(lastBanDate).Hours())
		waitHours := viper.GetInt("ban.wait.hours")
		if timeSinceBan > waitHours {
			msg := tgbotapi.NewMessage(user.TelegramUserID, "Maybe it's time to comeback?\nTap: /join")
			if _, err := bot.Request(msg); err != nil {
				log.Error("can't send private message", "error", err)
			}
			log.Info("sent a nudge to user", "name", user.GetName())
		}
	}
}

func CreateChart(bot *tgbotapi.BotAPI) {
	groups := users.GetGroupsWithUsers()
	for _, group := range groups {
		if len(group.Users) == 0 {
			continue
		}
		fileName := fmt.Sprintf("%d.png", group.ChatID)
		usersWorkouts, previousWeekWorkouts, leaders := collectUsersData(group)
		userNames := group.GetUserFixedNamesList()
		usersStringSlice := "'" + strings.Join(userNames, "', '") + "'"
		previousWorkoutsStringSlice := strings.Join(previousWeekWorkouts, ", ")
		workoutsStringSlice := strings.Join(usersWorkouts, ", ")
		chartConfig := createChartConfig(usersStringSlice, previousWorkoutsStringSlice, workoutsStringSlice)
		qc := createQuickChart(chartConfig)
		file, err := os.Create(fileName)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		qc.Write(file)

		// Get monthly standings
		monthlyLeaders := getMonthlyLeaders(group)

		msg := tgbotapi.NewPhoto(group.ChatID, tgbotapi.FilePath(fileName))
		caption := "Weekly summary:\n"

		// Add weekly leader info and select a winner
		var selectedWinner users.User

		if len(leaders) <= 1 {
			// No leaders or only the empty leader placeholder
			log.Debug("No leaders this week")
		} else if len(leaders) == 2 {
			// Only one real leader (the first is the empty placeholder)
			leader := leaders[1]
			selectedWinner = leader.User
			caption += fmt.Sprintf(
				"%s is the ⭐ with %d workouts!",
				leader.User.GetName(),
				leader.Workouts,
			)
			if err := leader.User.RegisterWeeklyLeaderEvent(); err != nil {
				log.Errorf("Error while registering weekly leader event: %s", err)
				sentry.CaptureException(err)
			}
		} else {
			// Multiple leaders (first is empty placeholder, rest are real)
			caption += fmt.Sprintf("⭐ Leaders of the week with %d workouts:\n",
				leaders[1].Workouts)

			// First announce all leaders
			for _, leader := range leaders[1:] {
				caption += leader.User.GetName() + " "
				if err := leader.User.RegisterWeeklyLeaderEvent(); err != nil {
					log.Errorf("Error while registering weekly leader event: %s", err)
					sentry.CaptureException(err)
				}
			}

			// Now determine the winner based on who reached the max workout count first
			selectedWinner = findEarliestLeader(leaders[1:], group.ChatID)

			caption += fmt.Sprintf("\n\n🏆 %s has been chosen as this week's winner (first to reach %d workouts)!",
				selectedWinner.GetName(), leaders[1].Workouts)
		}

		// Add monthly standings info
		if len(monthlyLeaders) > 0 {
			caption += "\n\nMonthly standings:"
			for i, leader := range monthlyLeaders {
				if i == 0 {
					caption += fmt.Sprintf("\n🥇 %s is leading with %d workouts",
						leader.User.GetName(), leader.Workouts)
				} else if i == 1 {
					caption += fmt.Sprintf("\n🥈 %s is in second place with %d workouts",
						leader.User.GetName(), leader.Workouts)
					break // Only show first and second place
				}
			}
		}

		msg.Caption = caption
		_, err = bot.Send(msg)
		if err != nil {
			log.Error(err)
			sentry.CaptureException(err)
		}

		// If we have a winner, ask them for a weekly message
		if selectedWinner.ID != 0 {
			// Send private message to the winner asking for their weekly message
			privateMsg := tgbotapi.NewMessage(
				selectedWinner.TelegramUserID,
				fmt.Sprintf("Congratulations on being this week's winner! 🏆\n\nPlease respond with your weekly message to the group. This message will be pinned and remembered."),
			)

			// Set up a force reply to make it clear they need to respond
			privateMsg.ReplyMarkup = tgbotapi.ForceReply{
				ForceReply: true,
				Selective:  false,
			}

			_, err = bot.Send(privateMsg)
			if err != nil {
				log.Error("Failed to send private message to weekly winner", "error", err)
				sentry.CaptureException(err)
			}

			// Send a message to the group announcing that the winner should send a message
			groupMsg := tgbotapi.NewMessage(
				group.ChatID,
				fmt.Sprintf("🎤 %s, as this week's winner, please share your weekly message/advice with the group!", selectedWinner.GetName()),
			)

			sentGroupMsg, err := bot.Send(groupMsg)
			if err != nil {
				log.Error("Failed to send group message about weekly winner message", "error", err)
				sentry.CaptureException(err)
			} else {
				// Pin this message to remind the winner
				pinChatMessageConfig := tgbotapi.PinChatMessageConfig{
					ChatID:              group.ChatID,
					MessageID:           sentGroupMsg.MessageID,
					DisableNotification: false,
				}

				_, err = bot.Request(pinChatMessageConfig)
				if err != nil {
					log.Error("Failed to pin message", "error", err)
					sentry.CaptureException(err)
				}
			}
		}
	}
}

func collectUsersData(group users.Group) (usersWorkouts, previousWeekWorkouts []string, leaders []Leader) {
	leaders = append(leaders, Leader{
		User:     users.User{},
		Workouts: 0,
	})
	for _, user := range group.Users {
		userPreviousWeekWorkouts := user.GetPreviousWeekWorkouts(group.ChatID)
		previousWeekWorkouts = append(previousWeekWorkouts, fmt.Sprint(len(userPreviousWeekWorkouts)))
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

func createChartConfig(usersStringSlice, previousWorkoutsSlice, workoutsStringSlice string) string {
	chartConfig := fmt.Sprintf(`{
		type: 'bar',
		data: {
			labels: [%s],
			datasets: [
				{
					label: 'Last Week',
					data: [%s]
				},
				{
					label: 'Workouts',
					data: [%s]
				},
			]
		}
	}`, usersStringSlice, previousWorkoutsSlice, workoutsStringSlice)
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

func ReportStandings(bot *tgbotapi.BotAPI) {
	// create stats message and send it to all groups
	statsMessage := "Here are the current standings:"
	groups := users.GetGroups()
	for _, group := range groups {
		stats := CreateStatsMessage(group.ChatID)
		msg := tgbotapi.NewMessage(group.ChatID, statsMessage+"\n"+stats)
		bot.Send(msg)
	}

}

func CreateStatsMessage(chatId int64) string {
	users := users.GetUsers(chatId)
	message := ""
	for _, user := range users {
		user.LoadWorkoutsThisCycle(chatId)
		workoutsStr := ""
		for range len(user.Workouts) {
			workoutsStr = workoutsStr + "🟩"
		}
		message = message +
			"\n" +
			fmt.Sprint(user.GetName()) +
			": " +
			workoutsStr
	}
	return message
}

func monthlyLeader(group users.Group) users.User {
	groupUsers, err := group.GetUsers()
	if err != nil {
		log.Error(err)
	}

	var topUser users.User
	for _, user := range groupUsers {
		user.LoadWorkoutsThisMonthlyCycle(group.ChatID)
		if topUser.ID == 0 {
			topUser = user
			continue
		}
		if len(user.Workouts) > len(topUser.Workouts) {
			topUser = user
		}
	}

	return topUser
}

func MonthlyReport(bot *tgbotapi.BotAPI) {
	log.Debug("starting monthly report")
	groups := users.GetGroupsWithUsers()
	for _, group := range groups {
		if len(group.Users) == 0 {
			continue
		}
		leader := monthlyLeader(group)
		if leader.ID == 0 {
			continue
		}
		leader.SetImmunity(true)
		msg := tgbotapi.NewMessage(group.ChatID, "")
		msg.Text = fmt.Sprintf("Monthly summary:\n🥇 %s has won this month with %d workouts!\nHe gets immunity 🛡️", leader.GetName(), len(leader.Workouts))
		_, err := bot.Send(msg)
		if err != nil {
			log.Error(err)
		}
	}
}

// getMonthlyLeaders returns a sorted list of leaders for the month
func getMonthlyLeaders(group users.Group) []Leader {
	var monthlyLeaders []Leader

	groupUsers, err := group.GetUsers()
	if err != nil {
		log.Error(err)
		return monthlyLeaders
	}

	for _, user := range groupUsers {
		user.LoadWorkoutsThisMonthlyCycle(group.ChatID)
		monthlyLeaders = append(monthlyLeaders, Leader{
			User:     user,
			Workouts: len(user.Workouts),
		})
	}

	// Sort leaders by workout count in descending order
	sort.Slice(monthlyLeaders, func(i, j int) bool {
		return monthlyLeaders[i].Workouts > monthlyLeaders[j].Workouts
	})

	return monthlyLeaders
}

// findEarliestLeader finds the leader who reached the max workout count first
func findEarliestLeader(leaders []Leader, chatID int64) users.User {
	if len(leaders) == 0 {
		return users.User{}
	}

	if len(leaders) == 1 {
		return leaders[0].User
	}

	var earliestLeader Leader = leaders[0]
	var earliestTime time.Time

	// Initialize with the first leader's last workout time
	lastWorkout, err := leaders[0].User.GetLastXWorkout(1, chatID)
	if err == nil {
		earliestTime = lastWorkout.CreatedAt
	} else {
		// If we can't get the last workout, just use current time
		earliestTime = time.Now()
	}

	// Check each leader's last workout time
	for _, leader := range leaders[1:] {
		lastWorkout, err := leader.User.GetLastXWorkout(1, chatID)
		if err != nil {
			log.Error("Failed to get last workout for user", "user", leader.User.GetName(), "error", err)
			continue
		}

		// If this leader's last workout is earlier than our current earliest,
		// they become the new earliest leader
		if lastWorkout.CreatedAt.Before(earliestTime) {
			earliestLeader = leader
			earliestTime = lastWorkout.CreatedAt
		}
	}

	return earliestLeader.User
}
