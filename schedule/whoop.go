package schedule

import (
	"fatbot/ai"
	"fatbot/db"
	"fatbot/state"
	"fatbot/users"

	"fatbot/whoop"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SyncWhoopWorkouts(bot *tgbotapi.BotAPI) {
	whoopUsers := users.GetWhoopUsers()
	for _, user := range whoopUsers {
		// Check if we have a recent workout to avoid spamming/API limits if needed
		// But simpler to just fetch new ones.

		accessToken, err := user.GetValidWhoopAccessToken()
		if err != nil {
			log.Errorf("Failed to get token for user %s: %s", user.GetName(), err)
			continue
		}

		// Fetch last 24h
		start := time.Now().Add(-24 * time.Hour)

		resp, err := whoop.GetWorkouts(accessToken, start, "")
		if err != nil {
			log.Errorf("Failed to fetch workouts for user %s: %s", user.GetName(), err)
			continue
		}

		for _, record := range resp.Records {
			if users.WorkoutExists(record.ID) {
				continue
			}

			// Check if ignored
			if _, err := state.Get("whoop:ignored:" + record.ID); err == nil {
				continue
			}

			// Check if pending
			if _, err := state.Get("whoop:pending:" + record.ID); err == nil {
				continue
			}

			// Check for duplicate image workout
			// Users usually upload AFTER the workout, so we check from Start-60m to End+60m
			margin := 60 * time.Minute
			existing, err := user.GetWorkoutInTimeRange(record.Start.Add(-margin), record.End.Add(margin))
			if err == nil && existing.ID != 0 && existing.WhoopID == "" {
				log.Infof("Skipping Whoop workout %s for user %s: matched existing workout %d", record.ID, user.GetName(), existing.ID)
				existing.WhoopID = record.ID
				db.DBCon.Save(&existing)
				continue
			}

			// Filter: Ignore short workouts (< 25 mins) or low strain (< 4.0)
			duration := record.End.Sub(record.Start)
			if duration < 25*time.Minute || record.Score.Strain < 4.0 {
				log.Debugf("Skipping workout %s: duration %.1f mins, strain %.1f", record.SportName, duration.Minutes(), record.Score.Strain)
				continue
			}

			if err := user.LoadGroups(); err != nil {
				log.Errorf("Failed to load groups for user %s: %s", user.GetName(), err)
				continue
			}

			lastWorkout, err := user.GetLastWorkout()
			// If we have a workout today, this is a bonus/secondary workout
			if err == nil && users.IsSameDay(lastWorkout.CreatedAt, record.Start) {
				// Send Question
				msg := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf("I detected a bonus workout: %s. Should this count as a separate workout?", record.SportName))
				yesBtn := tgbotapi.NewInlineKeyboardButtonData("Yes", fmt.Sprintf("whoop:yes:%s", record.ID))
				noBtn := tgbotapi.NewInlineKeyboardButtonData("No", fmt.Sprintf("whoop:no:%s", record.ID))
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(yesBtn, noBtn))
				bot.Send(msg)

				// Mark Pending
				state.SetWithTTL("whoop:pending:"+record.ID, "1", 86400) // 24h
				continue
			}

			// --- MAIN WORKOUT LOGIC (Not Bonus) ---
			for _, group := range user.Groups {
				// Calculate streak
				lastWorkout, err := user.GetLastXWorkout(1, group.ChatID)
				streak := 0

				if err == nil {
					daysDiff := int(record.Start.Sub(lastWorkout.CreatedAt).Hours() / 24)
					if daysDiff <= 1 {
						if lastWorkout.Streak > 0 {
							streak = lastWorkout.Streak + 1
						} else {
							streak = 2
						}
					}
				} else {
					// First ever workout
					streak = 1
				}

				workout := users.Workout{
					UserID:  user.ID,
					GroupID: group.ID,
					WhoopID: record.ID,
					Streak:  streak,
				}
				db.DBCon.Create(&workout)

				msg := tgbotapi.NewMessage(group.ChatID, fmt.Sprintf(
					"🏋️ %s just completed a %s workout!\n\nStrain: %.1f\nCalories: %.0f\nAvg HR: %d\nDuration: %.0f min\n\nStreak: %d",
					user.GetName(),
					record.SportName,
					record.Score.Strain,
					record.Score.Kilojoule/4.184, // KJ to Kcal
					record.Score.AverageHeartRate,
					duration.Minutes(),
					streak,
				))
				bot.Send(msg)

				// --- Additional Message (Image Upload Style) ---
				if err := user.LoadWorkoutsThisCycle(group.ChatID); err != nil {
					log.Errorf("Failed to load workouts for user %s: %s", user.GetName(), err)
				}

				ranks := users.GetRanks()
				var userRank users.Rank
				if user.Rank >= 0 && user.Rank < len(ranks) {
					userRank = ranks[user.Rank]
				} else {
					if len(ranks) > 0 {
						userRank = ranks[0]
					}
				}

				timeAgo := ""
				if !lastWorkout.CreatedAt.IsZero() {
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

				aiResponse := ai.GetAiWhoopResponse(record.SportName, record.Score.Strain, record.Score.Kilojoule/4.184, record.Score.AverageHeartRate, duration.Minutes())
				if aiResponse == "" {
					aiResponse = "Great work!"
				}

				var message string
				if lastWorkout.CreatedAt.IsZero() {
					message = fmt.Sprintf("%s nice work!\nThis is your first workout", user.GetName())
				} else {
					message = fmt.Sprintf("%s %s\nYour rank: %s\nLast workout: %s (%s)\nThis week: %d\n%s",
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

				newMsg := tgbotapi.NewMessage(group.ChatID, message)
				bot.Send(newMsg)
			}
			// Send PM to user to record video note - ONCE per workout
			pm := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf("Great job on your %s workout! 🏋️\n\nReply to this message with a video note to send it to all your groups.", record.SportName))
			bot.Send(pm)
		}
	}
}
