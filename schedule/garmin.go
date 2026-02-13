package schedule

import (
	"encoding/json"
	"fatbot/db"
	"fatbot/garmin"
	"fatbot/notify"
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func calculateStrain(avgHR int) float64 {
	if avgHR < 60 {
		return 0
	}
	// Simplified linear-ish mapping for a 1.0-21.0 scale
	// (AvgHR - 60) / (MaxHR - 60) * 21
	// Assuming MaxHR around 190
	maxHR := 190.0
	minHR := 60.0
	strain := (float64(avgHR) - minHR) / (maxHR - minHR) * 21.0
	if strain > 21.0 {
		strain = 21.0
	}
	if strain < 0 {
		strain = 0
	}
	return strain
}

func ProcessGarminActivity(bot *tgbotapi.BotAPI, user users.User, activity garmin.ActivityData) {
	// Normalize ID
	baseID := activity.SummaryID
	if idx := strings.Index(baseID, "-"); idx != -1 {
		baseID = baseID[:idx]
	}

	// 1. Check if already in DB
	if users.GarminWorkoutExists(baseID) {
		return
	}

	// 2. Check if recently processed (Atomic Lock)
	lockKey := "garmin:lock:" + baseID
	if _, err := state.Get(lockKey); err == nil {
		return
	}
	state.SetWithTTL(lockKey, "1", 30) // 30-second lock

	// 3. Check if ignored
	if _, err := state.Get("garmin:ignored:" + baseID); err == nil {
		return
	}

	// 4. Check if pending
	if _, err := state.Get("garmin:pending:" + baseID); err == nil {
		return
	}

	startTime := time.Unix(activity.StartTimeInSeconds, 0)
	duration := time.Duration(activity.DurationInSeconds) * time.Second
	endTime := startTime.Add(duration)

	// Check for duplicate image workout
	margin := 60 * time.Minute
	existing, err := user.GetWorkoutInTimeRange(startTime.Add(-margin), endTime.Add(margin))
	if err == nil && existing.ID != 0 && existing.GarminID == "" {
		log.Infof("Skipping Garmin activity %s for user %s: matched existing workout %d", activity.SummaryID, user.GetName(), existing.ID)
		existing.GarminID = activity.SummaryID
		db.DBCon.Save(&existing)
		return
	}

	// Filter: For TESTING, we allow > 30s. In production, this should be 25m.
	if duration < 30*time.Second {
		log.Debugf("Skipping Garmin activity %s: duration too short (%.1f seconds)", activity.ActivityName, duration.Seconds())
		return
	}

	if err := user.LoadGroups(); err != nil {
		log.Errorf("Failed to load groups for user %s: %s", user.GetName(), err)
		return
	}

	lastWorkout, err := user.GetLastWorkout()
	isBonus := err == nil && users.IsSameDay(lastWorkout.CreatedAt, startTime)
	// FOR TESTING: isSmall is always false to force posting to group
	isSmall := false

	if isBonus || isSmall {
		// Store activity data in state for the callback to use
		activityJSON, _ := json.Marshal(activity)
		state.SetWithTTL("garmin:data:"+activity.SummaryID, string(activityJSON), 86400)

		// Send Question
		msg := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf("I detected a Garmin activity: %s. Should this count as a workout?", activity.ActivityName))
		yesBtn := tgbotapi.NewInlineKeyboardButtonData("Yes", fmt.Sprintf("garmin:yes:%s", activity.SummaryID))
		noBtn := tgbotapi.NewInlineKeyboardButtonData("No", fmt.Sprintf("garmin:no:%s", activity.SummaryID))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(yesBtn, noBtn))
		bot.Send(msg)

		// Mark Pending
		state.SetWithTTL("garmin:pending:"+activity.SummaryID, "1", 86400) // 24h
		return
	}

	// --- MAIN WORKOUT LOGIC ---
	strain := calculateStrain(activity.AverageHeartRate)
	for _, group := range user.Groups {
		workout := users.Workout{
			UserID:   user.ID,
			GroupID:  group.ID,
			GarminID: activity.SummaryID,
		}
		db.DBCon.Create(&workout)
		notify.NotifyWorkout(bot, user, workout, activity.ActivityName, strain, activity.Calories, activity.AverageHeartRate, duration.Minutes())
	}
	notify.SendWorkoutPM(bot, user, activity.ActivityName)
}
