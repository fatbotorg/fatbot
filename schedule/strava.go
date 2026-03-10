package schedule

import (
	"encoding/json"
	"fatbot/db"
	"fatbot/notify"
	"fatbot/state"
	"fatbot/strava"
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ProcessStravaActivity processes a Strava activity webhook event
func ProcessStravaActivity(bot *tgbotapi.BotAPI, user users.User, activityID int64) {
	stravaID := fmt.Sprintf("%d", activityID)

	// 1. Check if already in DB
	if users.StravaWorkoutExists(stravaID) {
		log.Debugf("Strava activity %s already exists in DB", stravaID)
		return
	}

	// 2. Atomic dedup lock — SET NX ensures only one goroutine proceeds
	lockKey := "strava:lock:" + stravaID
	acquired, err := state.SetNX(lockKey, "1", 30)
	if err != nil {
		log.Errorf("Failed to acquire Strava lock for %s: %s", stravaID, err)
		return
	}
	if !acquired {
		log.Debugf("Strava activity %s already being processed (lock exists)", stravaID)
		return
	}

	// 3. Check if ignored
	if _, err := state.Get("strava:ignored:" + stravaID); err == nil {
		return
	}

	// 4. Check if pending
	if _, err := state.Get("strava:pending:" + stravaID); err == nil {
		return
	}

	// 5. Get valid access token
	accessToken, err := user.GetValidStravaAccessToken()
	if err != nil {
		log.Errorf("Failed to get valid Strava token for user %s: %s", user.GetName(), err)
		return
	}

	// 6. Fetch activity details from Strava API
	activity, err := strava.GetActivity(accessToken, activityID)
	if err != nil {
		log.Errorf("Failed to fetch Strava activity %d for user %s: %s", activityID, user.GetName(), err)
		return
	}

	// 7. Parse activity start time
	startTime, err := time.Parse(time.RFC3339, activity.StartDate)
	if err != nil {
		log.Errorf("Failed to parse Strava activity start time: %s", err)
		startTime = time.Now()
	}

	duration := time.Duration(activity.MovingTime) * time.Second
	endTime := startTime.Add(duration)

	// 8. Check for duplicate image workout (within time margin)
	margin := 60 * time.Minute
	existing, err := user.GetWorkoutInTimeRange(startTime.Add(-margin), endTime.Add(margin))
	if err == nil && existing.ID != 0 && existing.StravaID == "" {
		log.Infof("Skipping Strava activity %s for user %s: matched existing workout %d", stravaID, user.GetName(), existing.ID)
		existing.StravaID = stravaID
		db.DBCon.Save(&existing)
		return
	}

	// 9. Filter: Duration >= 25 minutes
	if duration < 1*time.Minute {
		log.Debugf("Skipping Strava activity %s: duration too short (%.1f min)", activity.Name, duration.Minutes())
		return
	}

	// 10. Load user groups
	if err := user.LoadGroups(); err != nil {
		log.Errorf("Failed to load groups for user %s: %s", user.GetName(), err)
		return
	}

	// 11. Check if bonus workout (same day as last workout)
	lastWorkout, err := user.GetLastWorkout()
	isBonus := err == nil && users.IsSameDay(lastWorkout.CreatedAt, startTime)

	if isBonus {
		// Store activity data in state for the callback to use
		activityJSON, _ := json.Marshal(activity)
		state.SetWithTTL("strava:data:"+stravaID, string(activityJSON), 86400) // 24h

		// Send question to user
		msg := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf(
			"I detected a Strava activity: %s (%s). This is your second workout today. Should this count as a workout?",
			activity.Name, activity.SportType))
		yesBtn := tgbotapi.NewInlineKeyboardButtonData("Yes", fmt.Sprintf("strava:yes:%s", stravaID))
		noBtn := tgbotapi.NewInlineKeyboardButtonData("No", fmt.Sprintf("strava:no:%s", stravaID))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(yesBtn, noBtn))
		bot.Send(msg)

		// Mark Pending
		state.SetWithTTL("strava:pending:"+stravaID, "1", 86400) // 24h
		return
	}

	// 12. Create workout for each group
	createStravaWorkout(bot, user, activity, stravaID)
}

// createStravaWorkout creates workout records and notifies groups
func createStravaWorkout(bot *tgbotapi.BotAPI, user users.User, activity *strava.ActivityData, stravaID string) {
	duration := time.Duration(activity.MovingTime) * time.Second

	var workouts []users.Workout
	for _, group := range user.Groups {
		workout := users.Workout{
			UserID:   user.ID,
			GroupID:  group.ID,
			StravaID: stravaID,
		}
		db.DBCon.Create(&workout)
		workouts = append(workouts, workout)

		notify.NotifyStravaWorkout(bot, user, workout, activity, duration.Minutes())
	}

	// If the user had pre-uploaded a photo, attach it automatically.
	// Otherwise fall back to the usual reply-with-photo prompt.
	if !notify.ApplyPendingPhoto(bot, user, workouts) {
		notify.SendWorkoutPM(bot, user, activity.Name)
	}
}

// ProcessStravaActivityFromCallback processes a Strava activity after user confirms via callback
func ProcessStravaActivityFromCallback(bot *tgbotapi.BotAPI, user users.User, stravaID string) error {
	// Check if already exists
	if users.StravaWorkoutExists(stravaID) {
		return fmt.Errorf("workout already registered")
	}

	// Get activity data from state
	activityDataJSON, err := state.Get("strava:data:" + stravaID)
	if err != nil {
		return fmt.Errorf("could not find Strava activity data in state: %s", err)
	}

	var activity strava.ActivityData
	if err := json.Unmarshal([]byte(activityDataJSON), &activity); err != nil {
		return fmt.Errorf("failed to unmarshal Strava activity data: %s", err)
	}

	// Clean up state
	state.ClearString("strava:data:" + stravaID)

	// Load groups
	if err := user.LoadGroups(); err != nil {
		return err
	}

	// Create workout
	createStravaWorkout(bot, user, &activity, stravaID)

	return nil
}
