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

type GroupScore struct {
	Group           users.Group
	TotalWorkouts   int
	AverageWorkouts float64
	UserCount       int
	IsNewBest       bool
	PreviousBest    float64
	IsFirstWeek     bool
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
	// Calculate group scores and rankings before sending reports
	groupScores := calculateGroupScores()
	totalActiveGroups := len(groupScores)

	// Build a map for quick lookup: chatId -> rank info including historical comparison
	type groupRankInfo struct {
		rank         int
		average      float64
		isNewBest    bool
		previousBest float64
		isFirstWeek  bool
	}
	rankMap := make(map[int64]groupRankInfo)
	for i, score := range groupScores {
		rankMap[score.Group.ChatID] = groupRankInfo{
			rank:         i + 1,
			average:      score.AverageWorkouts,
			isNewBest:    score.IsNewBest,
			previousBest: score.PreviousBest,
			isFirstWeek:  score.IsFirstWeek,
		}
	}

	groups := users.GetGroupsWithUsers()
	for _, group := range groups {
		if len(group.Users) == 0 {
			continue
		}

		// Skip groups with fewer than 4 members
		if len(group.Users) < 4 {
			log.Debug("Skipping weekly report for small group",
				"group_id", group.ChatID,
				"name", group.Title,
				"member_count", len(group.Users))
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

		if len(leaders) == 0 {
			// No leaders this week
			log.Debug("No leaders this week")
		} else if len(leaders) == 1 {
			// Only one leader
			leader := leaders[0]
			selectedWinner = leader.User
			caption += fmt.Sprintf(
				"%s is the ⭐ with %d workouts!",
				leader.User.GetName(),
				leader.Workouts,
			)
			if err := leader.User.RegisterWeeklyLeaderEvent(group.ChatID); err != nil {
				log.Errorf("Error while registering weekly leader event: %s", err)
				sentry.CaptureException(err)
			}
		} else {
			// Multiple leaders
			caption += fmt.Sprintf("⭐ Leaders of the week with %d workouts:\n",
				leaders[0].Workouts)

			// First announce all leaders
			for _, leader := range leaders {
				caption += leader.User.GetName() + " "
				if err := leader.User.RegisterWeeklyLeaderEvent(group.ChatID); err != nil {
					log.Errorf("Error while registering weekly leader event: %s", err)
					sentry.CaptureException(err)
				}
			}

			// Now determine the winner based on who reached the max workout count first
			selectedWinner = findEarliestLeader(leaders, group.ChatID)
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

		// Add group ranking info
		if rankInfo, ok := rankMap[group.ChatID]; ok && totalActiveGroups > 0 {
			caption += fmt.Sprintf("\n\nYour group averaged %.1f workouts per member this week. You're ranked %d/%d among active groups!",
				rankInfo.average, rankInfo.rank, totalActiveGroups)

			// Add historical comparison message
			if rankInfo.isFirstWeek {
				caption += "\nThis is your first recorded week! Your record has been set - try to break it next week!"
			} else if rankInfo.isNewBest {
				caption += "\nThis is your best week in recorded history!"
			} else {
				diff := rankInfo.previousBest - rankInfo.average
				caption += fmt.Sprintf("\nYou were %.1f points away from your best week ever (%.1f).", diff, rankInfo.previousBest)
			}
		}

		msg.Caption = caption
		_, err = bot.Send(msg)
		if err != nil {
			log.Error(err)
			sentry.CaptureException(err)
		}

		// If we have a winner, ask them for a weekly message directly in the group chat
		if selectedWinner.ID != 0 {
			// Create a mention that works even if user has no username
			userMention := fmt.Sprintf("[%s](tg://user?id=%d)", selectedWinner.GetName(), selectedWinner.TelegramUserID)

			groupMsg := tgbotapi.NewMessage(
				group.ChatID,
				fmt.Sprintf("🎤 %s, as this week's first leader, please share your weekly message as a reply to this message", userMention),
			)

			// Enable markdown for the mention to work
			groupMsg.ParseMode = "MarkdownV2"

			_, err := bot.Send(groupMsg)
			if err != nil {
				log.Error("Failed to send group message about weekly winner message", "error", err)
				sentry.CaptureException(err)
			}
		}
	}
}

func collectUsersData(group users.Group) (usersWorkouts, previousWeekWorkouts []string, leaders []Leader) {
	var maxWorkouts int = 0
	for i := range group.Users {
		user := &group.Users[i] // Get a pointer to the user in the slice

		userPreviousWeekWorkouts := user.GetPreviousWeekWorkouts(group.ChatID)
		previousWeekWorkouts = append(previousWeekWorkouts, fmt.Sprint(len(userPreviousWeekWorkouts)))

		// Use ReportCycle (last 7 days) instead of ThisCycle (which resets to 0 on report day)
		if err := user.LoadWorkoutsReportCycle(group.ChatID); err != nil {
			log.Error("Error loading workouts for user", "user_id", user.ID, "error", err)
			sentry.CaptureException(err)
			continue // Skip this user if workouts cannot be loaded
		}
		usersWorkouts = append(usersWorkouts, fmt.Sprint(len(user.Workouts)))

		workoutCount := len(user.Workouts)
		if workoutCount == 0 {
			continue
		}

		if workoutCount > maxWorkouts {
			// Found a new maximum, clear previous leaders
			leaders = []Leader{}
			maxWorkouts = workoutCount
		}

		// If this user has the current maximum workouts, add them as a leader
		if workoutCount == maxWorkouts {
			leaders = append(leaders, Leader{
				User:     *user, // Dereference the pointer for the struct field
				Workouts: workoutCount,
			})
		}
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

// calculateGroupScores calculates the average workouts per user for all active groups,
// compares with historical best, updates if new record, and returns sorted by average
// (descending), with ties broken by user count (descending)
func calculateGroupScores() []GroupScore {
	groups := users.GetGroupsWithUsers()
	var scores []GroupScore

	for _, group := range groups {
		// Skip groups with fewer than 4 members (not active)
		if len(group.Users) < 4 {
			continue
		}

		totalWorkouts := 0
		for i := range group.Users {
			user := &group.Users[i]
			if err := user.LoadWorkoutsReportCycle(group.ChatID); err != nil {
				log.Error("Error loading workouts for group score", "user_id", user.ID, "error", err)
				continue
			}
			totalWorkouts += len(user.Workouts)
		}

		userCount := len(group.Users)
		average := float64(totalWorkouts) / float64(userCount)

		// Compare with historical best and update if needed
		isNewBest, previousBest, err := group.UpdateBestAverageIfHigher(average)
		if err != nil {
			log.Error("Error updating best average for group", "group_id", group.ChatID, "error", err)
		}

		// Check if this is the first week (no previous record)
		isFirstWeek := previousBest == 0

		scores = append(scores, GroupScore{
			Group:           group,
			TotalWorkouts:   totalWorkouts,
			AverageWorkouts: average,
			UserCount:       userCount,
			IsNewBest:       isNewBest,
			PreviousBest:    previousBest,
			IsFirstWeek:     isFirstWeek,
		})
	}

	// Sort by average (descending), ties broken by user count (descending)
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].AverageWorkouts == scores[j].AverageWorkouts {
			return scores[i].UserCount > scores[j].UserCount
		}
		return scores[i].AverageWorkouts > scores[j].AverageWorkouts
	})

	return scores
}

func ReportStandings(bot *tgbotapi.BotAPI) {
	groups := users.GetGroups()
	for _, group := range groups {
		groupWithUsers := users.GetGroupWithUsers(group.ChatID)

		if len(groupWithUsers.Users) < 4 {
			log.Debug("Skipping standings report for small group",
				"group_id", group.ChatID,
				"name", group.Title,
				"member_count", len(groupWithUsers.Users))
			continue
		}

		message := buildPowerRankingsMessage(groupWithUsers)
		msg := tgbotapi.NewMessage(group.ChatID, message)
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

type WeeklyStats struct {
	User             users.User
	ThisWeekWorkouts int
	LastWeekWorkouts int
	Improvement      int
	DaysLeftToWin    string
}

func buildPowerRankingsMessage(group users.Group) string {
	stats := collectWeeklyStats(group)

	if len(stats) == 0 {
		return "📊 Mid-Week Power Rankings 📊\n\nNo workout data available yet this week."
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].ThisWeekWorkouts == stats[j].ThisWeekWorkouts {
			return stats[i].Improvement > stats[j].Improvement
		}
		return stats[i].ThisWeekWorkouts > stats[j].ThisWeekWorkouts
	})

	message := "📊 Mid-Week Power Rankings 📊\n\n"
	message += "🏆 Current Leaderboard:\n"

	maxWorkouts := stats[0].ThisWeekWorkouts
	leadersCount := 0
	for _, s := range stats {
		if s.ThisWeekWorkouts == maxWorkouts {
			leadersCount++
		}
	}

	for i, s := range stats {
		var position string
		if i == 0 {
			position = "🥇"
		} else if i == 1 {
			position = "🥈"
		} else if i == 2 {
			position = "🥉"
		} else {
			position = fmt.Sprintf("%d.", i+1)
		}

		improvementStr := ""
		if s.Improvement > 0 {
			improvementStr = fmt.Sprintf(" 📈+%d", s.Improvement)
		} else if s.Improvement < 0 {
			improvementStr = fmt.Sprintf(" 📉%d", s.Improvement)
		}

		message += fmt.Sprintf("%s %s: %d workouts%s\n",
			position, s.User.GetName(), s.ThisWeekWorkouts, improvementStr)
	}

	comebackPlayer := findComebackPlayer(stats)
	if comebackPlayer != nil {
		message += fmt.Sprintf("\n🔥 Comeback Player: %s (+%d vs last week!)\n",
			comebackPlayer.User.GetName(), comebackPlayer.Improvement)
	}

	if leadersCount > 1 && maxWorkouts > 0 {
		message += fmt.Sprintf("\n⚡ CLOSE RACE! %d players tied at %d workouts!\n",
			leadersCount, maxWorkouts)
	}

	closeContenders := findCloseContenders(stats, maxWorkouts)
	if len(closeContenders) > 0 {
		message += "\n💪 Win Probability:\n"
		for _, contender := range closeContenders {
			message += contender
		}
	}

	return message
}

func collectWeeklyStats(group users.Group) []WeeklyStats {
	var stats []WeeklyStats

	for i := range group.Users {
		user := &group.Users[i] // Get a pointer to the user in the slice

		lastWeekWorkouts := user.GetPreviousWeekWorkouts(group.ChatID)

		if err := user.LoadWorkoutsThisCycle(group.ChatID); err != nil {
			log.Error("Error loading workouts for user", "user_id", user.ID, "error", err)
			sentry.CaptureException(err)
			continue // Skip this user if workouts cannot be loaded
		}
		thisWeekWorkouts := user.Workouts

		improvement := len(thisWeekWorkouts) - len(lastWeekWorkouts)

		stats = append(stats, WeeklyStats{
			User:             *user, // Dereference the pointer for the struct field
			ThisWeekWorkouts: len(thisWeekWorkouts),
			LastWeekWorkouts: len(lastWeekWorkouts),
			Improvement:      improvement,
		})
	}

	return stats
}

func findComebackPlayer(stats []WeeklyStats) *WeeklyStats {
	var bestComeback *WeeklyStats
	maxImprovement := 0

	for i := range stats {
		if stats[i].Improvement > maxImprovement && stats[i].Improvement >= 2 {
			maxImprovement = stats[i].Improvement
			bestComeback = &stats[i]
		}
	}

	return bestComeback
}

func findCloseContenders(stats []WeeklyStats, maxWorkouts int) []string {
	if maxWorkouts == 0 {
		return nil
	}

	var contenders []string
	daysLeft := 7 - int(time.Now().Weekday())
	if daysLeft <= 0 || daysLeft >= 5 {
		return nil
	}

	for _, s := range stats {
		gap := maxWorkouts - s.ThisWeekWorkouts

		if gap == 0 {
			contenders = append(contenders,
				fmt.Sprintf("• %s: Leading! One more workout seals it 🏆\n", s.User.GetName()))
		} else if gap == 1 {
			contenders = append(contenders,
				fmt.Sprintf("• %s: 1 workout behind - still in the game! 🎯\n", s.User.GetName()))
		} else if gap == 2 && daysLeft >= 2 {
			contenders = append(contenders,
				fmt.Sprintf("• %s: 2 workouts needed - you've got time! ⏰\n", s.User.GetName()))
		}
	}

	return contenders
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

		// Skip groups with fewer than 4 members
		if len(group.Users) < 4 {
			log.Debug("Skipping monthly report for small group",
				"group_id", group.ChatID,
				"name", group.Title,
				"member_count", len(group.Users))
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
