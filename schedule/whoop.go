package schedule

import (
	"fatbot/db"
	"fatbot/notify"
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
				continue
			}

			if err := user.LoadGroups(); err != nil {
				log.Errorf("Failed to load groups for user %s: %s", user.GetName(), err)
				continue
			}

			lastWorkout, err := user.GetLastWorkout()
			isBonus := err == nil && users.IsSameDay(lastWorkout.CreatedAt, record.Start)
			isSmall := record.Score.Strain < 10.0

			// If we have a workout today, this is a bonus/secondary workout
			// Or if it is a small workout (low strain)
			if isBonus || isSmall {
				// Send Question
				msg := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf("I detected a workout: %s. Should this count as a workout?", record.SportName))
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
				workout := users.Workout{
					UserID:  user.ID,
					GroupID: group.ID,
					WhoopID: record.ID,
				}
				db.DBCon.Create(&workout)
						notify.NotifyWorkout(bot, user, workout, record.SportName, record.Score.Strain, record.Score.Kilojoule/4.184, record.Score.AverageHeartRate, duration.Minutes(), 0, "", "")
					}
					notify.SendWorkoutPM(bot, user, record.SportName)
				}	}
}
