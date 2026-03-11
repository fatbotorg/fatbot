package notify

import (
	"fatbot/ai"
	"fatbot/db"
	"fatbot/state"
	"fatbot/strava"
	"fatbot/users"
	"fatbot/whoop"
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
	sentMsg, sendErr := bot.Send(msg)
	if sendErr == nil && workout.WhoopID != "" {
		// Store the message ID so we can edit it later if the workout is updated
		// (e.g., strain adjusted via Strength Trainer in the Whoop app)
		workout.NotifyMessageID = sentMsg.MessageID
		workout.NotifyChatID = group.ChatID
		db.DBCon.Save(&workout)
	}

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
	// Skip the "reply with a photo" prompt if the user already has a photo
	// pre-saved for this workout — it was already applied by ApplyPendingPhoto.
	if _, err := state.GetPendingPhoto(user.TelegramUserID); err == nil {
		return
	}
	pm := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf("Great job on your %s workout!\n\nReply to this message with a photo to send it to all your groups.", sportName))
	bot.Send(pm)
}

// ApplyPendingPhoto checks whether the user has a pending pre-workout photo stored in Redis.
// If found, it attaches the photo to each of the provided workouts, forwards it to the
// corresponding group chats, and clears the pending photo from Redis.
// Returns true if a pending photo was found and applied.
func ApplyPendingPhoto(bot *tgbotapi.BotAPI, user users.User, workouts []users.Workout) bool {
	fileID, err := state.GetPendingPhoto(user.TelegramUserID)
	if err != nil {
		// No pending photo — nothing to do.
		return false
	}

	// Attach photo to each workout record
	for i := range workouts {
		workouts[i].PhotoFileID = fileID
		db.DBCon.Save(&workouts[i])
	}

	// Forward the photo to each group the workout was posted to
	for _, workout := range workouts {
		group, err := users.GetGroupByID(workout.GroupID)
		if err != nil {
			log.Errorf("ApplyPendingPhoto: failed to get group %d: %s", workout.GroupID, err)
			continue
		}
		photoMsg := tgbotapi.NewPhoto(group.ChatID, tgbotapi.FileID(fileID))
		if _, err := bot.Send(photoMsg); err != nil {
			log.Errorf("ApplyPendingPhoto: failed to send photo to group %d: %s", group.ChatID, err)
		}
	}

	// Clear the pending photo so it isn't reused
	if err := state.ClearPendingPhoto(user.TelegramUserID); err != nil {
		log.Errorf("ApplyPendingPhoto: failed to clear pending photo for user %d: %s", user.TelegramUserID, err)
	}

	return true
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
	msgText := fmt.Sprintf("<b>STRAVA</b>\n\n")
	msgText += fmt.Sprintf("🏋️ %s just completed a workout!\n", user.GetName())
	msgText += fmt.Sprintf("Activity: %s\n", activity.Name)

	// Distance
	if activity.Distance > 0 {
		distanceKm := activity.Distance / 1000.0
		msgText += fmt.Sprintf("Distance: %.2f km\n", distanceKm)

		// Pace (for running/cycling activities)
		if durationMins > 0 && distanceKm > 0 {
			pace := durationMins / distanceKm
			paceMins := int(pace)
			paceSecs := int((pace - float64(paceMins)) * 60)
			msgText += fmt.Sprintf("Pace: %d:%02d /km\n", paceMins, paceSecs)
		}
	}

	// Duration
	hours := int(durationMins) / 60
	mins := int(durationMins) % 60
	if hours > 0 {
		msgText += fmt.Sprintf("Time: %d:%02d\n", hours, mins)
	} else {
		msgText += fmt.Sprintf("Time: %d min\n", mins)
	}

	// Heart Rate
	if activity.AverageHeartrate > 0 {
		msgText += fmt.Sprintf("Avg HR: %.0f bpm\n", activity.AverageHeartrate)
	}

	// Calories
	if activity.Calories > 0 {
		msgText += fmt.Sprintf("Calories: %.0f kcal\n", activity.Calories)
	}

	// Suffer Score / Relative Effort (if available - Strava Premium feature)
	if activity.SufferScore != nil && *activity.SufferScore > 0 {
		strainEquiv := strava.SufferScoreToStrain(*activity.SufferScore)
		msgText += fmt.Sprintf("Relative Effort: %.0f (~%.1f strain)\n", *activity.SufferScore, strainEquiv)
	}

	// Device
	if activity.DeviceName != "" {
		msgText += fmt.Sprintf("Device: %s\n", activity.DeviceName)
	}

	msgText += fmt.Sprintf("\n<i>Powered by Strava</i> | <a href=\"https://www.strava.com/activities/%d\">View on Strava</a>", activity.ID)

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

// EditWhoopNotification edits an existing group notification message with updated Whoop workout data.
// This is called when a workout.updated webhook arrives for a workout that was already reported
// (e.g., the user adjusted strain via Strength Trainer in the Whoop app).
func EditWhoopNotification(bot *tgbotapi.BotAPI, user users.User, workout users.Workout, record *whoop.WorkoutData) {
	if workout.NotifyMessageID == 0 || workout.NotifyChatID == 0 {
		return
	}

	duration := record.End.Sub(record.Start)

	// Rebuild the notification text in the same format as NotifyWorkout (Whoop path)
	msgText := fmt.Sprintf(
		"🏋️ %s just completed a %s workout!\n\n",
		user.GetName(),
		record.SportName,
	)
	if record.Score.Strain > 0 {
		msgText += fmt.Sprintf("Strain: %.1f\n", record.Score.Strain)
	}
	msgText += fmt.Sprintf("Calories: %.0f\nAvg HR: %d\nDuration: %.0f min",
		record.Score.Kilojoule/4.184,
		record.Score.AverageHeartRate,
		duration.Minutes(),
	)

	edit := tgbotapi.NewEditMessageText(workout.NotifyChatID, workout.NotifyMessageID, msgText)
	edit.ParseMode = "HTML"
	if _, err := bot.Send(edit); err != nil {
		log.Errorf("Failed to edit Whoop notification (chat=%d, msg=%d): %s", workout.NotifyChatID, workout.NotifyMessageID, err)
	}
}
