package notify

import (
	"fatbot/ai"
	"fatbot/db"
	"fatbot/strava"
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func NotifyWorkout(bot *tgbotapi.BotAPI, user users.User, workout users.Workout, sportName string, strain float64, calories float64, avgHR int, durationMins float64, distance float64, deviceName string, activityType string) {
	group, _ := users.GetGroupByID(workout.GroupID)

	// Calculate streak for this specific group
	lastWorkout, err := user.GetLastXWorkout(2, group.ChatID) // 2 because the current one is already in DB
	streak := 0

	if err == nil {
		if users.IsTodayOrWasYesterday(lastWorkout.CreatedAt) {
			if lastWorkout.Streak > 0 {
				streak = lastWorkout.Streak + 1
			} else {
				streak = 2
			}
		}
	}

	workout.Streak = streak
	db.DBCon.Save(&workout)

	// Main Announcement
	var msgText string
	if workout.GarminID != "" {
		msgText = fmt.Sprintf("<b>GARMIN</b>\n\n")
		msgText += fmt.Sprintf("🏋️ %s just completed a workout!\n\n", user.GetName())
		displayActivityType := activityType
		if displayActivityType == "" {
			displayActivityType = sportName
		}
		msgText += fmt.Sprintf("• Activity Type: %s\n", displayActivityType)
		if distance > 0 {
			distanceKm := distance / 1000.0
			msgText += fmt.Sprintf("• Distance: %.2f km\n", distanceKm)
			if durationMins > 0 {
				pace := durationMins / distanceKm
				paceMins := int(pace)
				paceSecs := int((pace - float64(paceMins)) * 60)
				msgText += fmt.Sprintf("• Pace/Speed: %d:%02d min/km\n", paceMins, paceSecs)
			}
		}
		msgText += fmt.Sprintf("• Time: %.0f min\n", durationMins)
		msgText += fmt.Sprintf("• Average HR: %d bpm\n", avgHR)
		if calories > 0 {
			msgText += fmt.Sprintf("• Calories: %.0f kcal\n", calories)
		}
		if deviceName != "" {
			msgText += fmt.Sprintf("• Device Model: %s\n", deviceName)
		}
		msgText += "\n<i>Data provided by Garmin</i>"
	} else {
		msgText = fmt.Sprintf(
			"🏋️ %s just completed a %s workout!\n\n",
			user.GetName(),
			sportName,
		)
		if strain > 0 {
			msgText += fmt.Sprintf("Strain: %.1f\n", strain)
		}
		msgText += fmt.Sprintf("Calories: %.0f\nAvg HR: %d\nDuration: %.0f min",
			calories,
			avgHR,
			durationMins,
		)
	}
	msg := tgbotapi.NewMessage(group.ChatID, msgText)
	msg.ParseMode = "HTML"
	bot.Send(msg)

	// Detailed Stats & Ranks
	if err := user.LoadWorkoutsThisCycle(group.ChatID); err != nil {
		log.Errorf("Failed to load workouts for user %s: %s", user.GetName(), err)
	}

	ranks := users.GetRanks()
	var userRank users.Rank
	if user.Rank >= 0 && user.Rank < len(ranks) {
		userRank = ranks[user.Rank]
	} else if len(ranks) > 0 {
		userRank = ranks[0]
	}

	timeAgo := ""
	if err == nil && !lastWorkout.CreatedAt.IsZero() {
		hours := time.Now().Sub(lastWorkout.CreatedAt).Hours()
		if int(hours/24) == 0 {
			timeAgo = fmt.Sprintf("%d hours ago", int(hours))
		} else {
			days := int(hours / 24)
			timeAgo = fmt.Sprintf("%d days and %d hours ago", days, int(hours)-days*24)
		}
	}

	var streakMessage string
	if streak > 0 {
		streakSigns := ""
		for i := 0; i < streak; i++ {
			streakSigns += "👑"
		}
		streakMessage = fmt.Sprintf("%d in a row! %s %s", streak, streakSigns, users.GetRandomStreakMessage())
	}

	aiResponse := ai.GetAiWhoopResponse(sportName, strain, calories, avgHR, durationMins)
	if aiResponse == "" {
		aiResponse = "Great work!"
	}

	var statsMessage string
	if err != nil || lastWorkout.CreatedAt.IsZero() {
		statsMessage = fmt.Sprintf("%s nice work!\nThis is your first workout", user.GetName())
	} else {
		statsMessage = fmt.Sprintf("%s %s\nYour rank: %s\nLast workout: %s (%s)\nThis week: %d\n%s",
			user.GetName(),
			aiResponse,
			fmt.Sprintf("%s %s (%d/%d)",
				userRank.Name,
				userRank.Emoji,
				user.Rank,
				len(ranks)),
			lastWorkout.CreatedAt.Weekday(),
			timeAgo,
			len(user.Workouts),
			streakMessage,
		)
	}
	if workout.GarminID != "" {
		statsMessage += "\n\n<i>Data provided by Garmin</i>"
	}
	msg = tgbotapi.NewMessage(group.ChatID, statsMessage)
	msg.ParseMode = "HTML"
	bot.Send(msg)
}

func SendWorkoutPM(bot *tgbotapi.BotAPI, user users.User, sportName string) {
	pm := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf("Great job on your %s workout!\n\nReply to this message with a photo to send it to all your groups.", sportName))
	bot.Send(pm)
}

// NotifyStravaWorkout sends a Strava-specific workout notification to the group
func NotifyStravaWorkout(bot *tgbotapi.BotAPI, user users.User, workout users.Workout, activity *strava.ActivityData, durationMins float64) {
	group, _ := users.GetGroupByID(workout.GroupID)

	// Calculate streak for this specific group
	lastWorkout, err := user.GetLastXWorkout(2, group.ChatID) // 2 because the current one is already in DB
	streak := 0

	if err == nil {
		if users.IsTodayOrWasYesterday(lastWorkout.CreatedAt) {
			if lastWorkout.Streak > 0 {
				streak = lastWorkout.Streak + 1
			} else {
				streak = 2
			}
		}
	}

	workout.Streak = streak
	db.DBCon.Save(&workout)

	// Build Strava-specific message
	msgText := "<b>━━━━━━━━ STRAVA ━━━━━━━━</b>\n"
	msgText += fmt.Sprintf("🏃 <b>%s</b>\n", activity.Name)
	msgText += "━━━━━━━━━━━━━━━━━━━━━━━\n"

	// Distance
	if activity.Distance > 0 {
		distanceKm := activity.Distance / 1000.0
		msgText += fmt.Sprintf("📏 Distance: %.2f km\n", distanceKm)

		// Pace (for running/cycling activities)
		if durationMins > 0 && distanceKm > 0 {
			pace := durationMins / distanceKm
			paceMins := int(pace)
			paceSecs := int((pace - float64(paceMins)) * 60)
			msgText += fmt.Sprintf("⏱ Pace: %d:%02d /km\n", paceMins, paceSecs)
		}
	}

	// Duration
	hours := int(durationMins) / 60
	mins := int(durationMins) % 60
	if hours > 0 {
		msgText += fmt.Sprintf("⏳ Time: %d:%02d\n", hours, mins)
	} else {
		msgText += fmt.Sprintf("⏳ Time: %d min\n", mins)
	}

	// Heart Rate
	if activity.AverageHeartrate > 0 {
		msgText += fmt.Sprintf("❤️ Avg HR: %.0f bpm\n", activity.AverageHeartrate)
	}

	// Calories
	if activity.Calories > 0 {
		msgText += fmt.Sprintf("🔥 Calories: %.0f kcal\n", activity.Calories)
	}

	// Suffer Score (if available - Strava Premium feature)
	if activity.SufferScore != nil && *activity.SufferScore > 0 {
		strainEquiv := strava.SufferScoreToStrain(*activity.SufferScore)
		msgText += fmt.Sprintf("😰 Suffer Score: %d (~%.1f strain)\n", *activity.SufferScore, strainEquiv)
	}

	// Device
	if activity.DeviceName != "" {
		msgText += fmt.Sprintf("📱 Device: %s\n", activity.DeviceName)
	}

	msgText += "━━━━━━━━━━━━━━━━━━━━━━━\n"
	msgText += fmt.Sprintf("<i>Powered by Strava</i> | <a href=\"https://www.strava.com/activities/%d\">View on Strava</a>", activity.ID)

	msg := tgbotapi.NewMessage(group.ChatID, msgText)
	msg.ParseMode = "HTML"
	bot.Send(msg)

	// Detailed Stats & Ranks
	if err := user.LoadWorkoutsThisCycle(group.ChatID); err != nil {
		log.Errorf("Failed to load workouts for user %s: %s", user.GetName(), err)
	}

	ranks := users.GetRanks()
	var userRank users.Rank
	if user.Rank >= 0 && user.Rank < len(ranks) {
		userRank = ranks[user.Rank]
	} else if len(ranks) > 0 {
		userRank = ranks[0]
	}

	timeAgo := ""
	if err == nil && !lastWorkout.CreatedAt.IsZero() {
		hours := time.Now().Sub(lastWorkout.CreatedAt).Hours()
		if int(hours/24) == 0 {
			timeAgo = fmt.Sprintf("%d hours ago", int(hours))
		} else {
			days := int(hours / 24)
			timeAgo = fmt.Sprintf("%d days and %d hours ago", days, int(hours)-days*24)
		}
	}

	var streakMessage string
	if streak > 0 {
		streakSigns := ""
		for i := 0; i < streak; i++ {
			streakSigns += "👑"
		}
		streakMessage = fmt.Sprintf("%d in a row! %s %s", streak, streakSigns, users.GetRandomStreakMessage())
	}

	// Use the activity sport type for AI response
	sportName := activity.SportType
	if sportName == "" {
		sportName = activity.Type
	}

	aiResponse := ai.GetAiWhoopResponse(sportName, 0, activity.Calories, int(activity.AverageHeartrate), durationMins)
	if aiResponse == "" {
		aiResponse = "Great work!"
	}

	var statsMessage string
	if err != nil || lastWorkout.CreatedAt.IsZero() {
		statsMessage = fmt.Sprintf("%s nice work!\nThis is your first workout", user.GetName())
	} else {
		statsMessage = fmt.Sprintf("%s %s\nYour rank: %s\nLast workout: %s (%s)\nThis week: %d\n%s",
			user.GetName(),
			aiResponse,
			fmt.Sprintf("%s %s (%d/%d)",
				userRank.Name,
				userRank.Emoji,
				user.Rank,
				len(ranks)),
			lastWorkout.CreatedAt.Weekday(),
			timeAgo,
			len(user.Workouts),
			streakMessage,
		)
	}
	statsMessage += "\n\n<i>Powered by Strava</i>"

	msg = tgbotapi.NewMessage(group.ChatID, statsMessage)
	msg.ParseMode = "HTML"
	bot.Send(msg)
}
