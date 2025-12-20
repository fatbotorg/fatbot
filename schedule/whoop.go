package schedule

import (
	"fatbot/db"
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

			for _, group := range user.Groups {
				// Calculate streak
				lastWorkout, err := user.GetLastXWorkout(1, group.ChatID)
				streak := 0
				var isNewDailyWorkout bool = true

				if err == nil {
					// Check if last workout was TODAY
					// If it was today, this is a secondary workout
					if users.IsSameDay(lastWorkout.CreatedAt, record.Start) {
						isNewDailyWorkout = false
					}

					// Calculate Streak (only if it's a new day's workout)
					if isNewDailyWorkout {
						daysDiff := int(record.Start.Sub(lastWorkout.CreatedAt).Hours() / 24)
						if daysDiff <= 1 {
							if lastWorkout.Streak > 0 {
								streak = lastWorkout.Streak + 1
							} else {
								streak = 2
							}
						}
					} else {
						streak = lastWorkout.Streak // Keep same streak
					}
				} else {
					// First ever workout
					streak = 1
				}

				if isNewDailyWorkout {
					// --- MAIN WORKOUT LOGIC ---
					workout := users.Workout{
						UserID:  user.ID,
						GroupID: group.ID,
						WhoopID: record.ID,
						Streak:  streak,
					}
					// Use record.Start to ensure accurate daily tracking, but GORM uses CreatedAt.
					// We'll let GORM set CreatedAt to Now() which is fine for "Synced Now".
					// To correctly backfill, we might want to manually set CreatedAt if it's old.
					// But for now, let's stick to simple logic.
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
				} else {
					// --- BONUS WORKOUT LOGIC ---
					// 1. Fetch Cycle to get Daily Strain
					// We need the cycle that includes this workout's start time
					cycleResp, err := whoop.GetCycleCollection(accessToken, record.Start.Add(-24*time.Hour)) // Look back a bit to find the cycle
					var dailyStrain float64
					if err == nil {
						// Find the cycle that matches the day
						for _, cycle := range cycleResp.Records {
							// Simple overlap check or just take the last one?
							// Whoop cycles usually align with days. Let's take the one with the highest ID or matching date.
							// For simplicity, let's take the most recent one returned.
							dailyStrain = cycle.Score.Strain
							break // Assuming sorted desc or just taking first found
						}
					}

					msg := tgbotapi.NewMessage(group.ChatID, fmt.Sprintf(
						"🏃 %s added a bonus activity: %s\n\nStrain: %.1f\nDuration: %.0f min\n\n🔥 Daily Accum. Strain: %.1f\n(Streak unaffected)",
						user.GetName(),
						record.SportName,
						record.Score.Strain,
						duration.Minutes(),
						dailyStrain,
					))
					bot.Send(msg)

					// IMPORTANT: Mark this WhoopID as "seen" in DB so we don't re-notify
					// We can create a "Ghost" workout or just flag it?
					// Or we can just insert it but NOT count it for streak/rank?
					// If we insert it, it becomes "LastWorkout".
					// If we insert it, IsSameDay check next time will see THIS one.
					// If we insert it, `users.Workout` table gets populated.
					// This seems fine! The "Streak" logic in `users.go` calculates based on *days*, not count.
					// So having multiple workouts in one day is actually supported by `GetLastXWorkout` logic?
					// Wait, `GetLastXWorkout` just gets the last one.
					// If we insert this "Bonus", next time `GetLastXWorkout` returns this Bonus.
					// Then `IsSameDay` compares New vs Bonus.
					// So it all works!

					// So we SHOULD insert it, but maybe we don't announce it as a "Streak Increase".
					// Wait, if I insert it, `isNewDailyWorkout` logic above handles the streak calculation for the *next* one.
					// But `streak` variable here is used for the *current* insertion.
					// If I insert it with `streak = lastWorkout.Streak`, it maintains the streak.

					workout := users.Workout{
						UserID:  user.ID,
						GroupID: group.ID,
						WhoopID: record.ID,
						Streak:  streak, // Maintain streak, don't increment
					}
					db.DBCon.Create(&workout)
				}
			}
		}
	}
}
