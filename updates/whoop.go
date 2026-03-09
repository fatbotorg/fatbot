package updates

import (
	"encoding/json"
	"fatbot/db"
	"fatbot/notify"
	"fatbot/state"
	"fatbot/users"
	"fatbot/whoop"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

func HandleWhoopCommand(fatBotUpdate FatBotUpdate) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.FromChat().ID, "")
	user, err := users.GetUserById(fatBotUpdate.Update.SentFrom().ID)
	if err != nil {
		return msg, err
	}

	state := fmt.Sprintf("fatbot-%d-%d", user.TelegramUserID, time.Now().Unix())
	// Force scope update by changing state prefix or just rely on the new scope constant in client
	authURL := whoop.GetAuthURL(state)
	log.Infof("Whoop Auth URL: %s", authURL)
	msg.Text = "Connect your Whoop account to automatically sync workouts."
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Connect Whoop", authURL),
		),
	)
	return msg, nil
}

func HandleWhoopCallback(w http.ResponseWriter, r *http.Request) {
	log.Infof("Whoop Callback received: %s", r.URL.String())
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	token, err := whoop.ExchangeToken(code)
	if err != nil {
		log.Errorf("Failed to exchange token: %s", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	parts := strings.Split(state, "-")
	if len(parts) < 2 {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	// parts[1] is the user ID based on "fatbot-USERID-TIMESTAMP"
	user, err := users.GetUserByState(parts[1])
	if err != nil {
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	if err := user.UpdateWhoopToken(token); err != nil {
		http.Error(w, "Failed to save token", http.StatusInternalServerError)
		return
	}

	// Fetch Whoop profile to store the Whoop user ID (needed for webhook lookups)
	profile, err := whoop.GetUserProfile(token.AccessToken)
	if err != nil {
		log.Errorf("Failed to fetch Whoop profile for user %s: %s", user.GetName(), err)
		// Non-fatal: webhooks won't work for this user but polling still will
	} else {
		user.WhoopUserID = profile.UserID
		db.DBCon.Save(&user)
	}

	log.Infof("Whoop connected for user %s", user.GetName())

	// Notify user via Telegram
	if GlobalBot != nil {
		msg := tgbotapi.NewMessage(user.TelegramUserID,
			"Your Whoop account has been connected successfully!\n\n"+
				"Your workouts will now be automatically synced. "+
				"Note: Any previous Strava or Garmin integration has been disconnected.")
		GlobalBot.Send(msg)
	}

	fmt.Fprint(w, "Whoop connected successfully! You can close this window.")
}

// HandleWhoopWebhook handles incoming Whoop webhook events (workout.updated, workout.deleted, etc.)
func HandleWhoopWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read Whoop webhook body: %s", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	log.Infof("Raw Whoop Webhook Body: %s", string(bodyBytes))

	// Validate webhook signature
	signature := r.Header.Get("X-WHOOP-Signature")
	timestamp := r.Header.Get("X-WHOOP-Signature-Timestamp")
	clientSecret := viper.GetString("whoop.client_secret")

	if signature != "" && clientSecret != "" {
		if !whoop.ValidateWebhookSignature(bodyBytes, timestamp, signature, clientSecret) {
			log.Error("Whoop webhook signature validation failed")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	var event whoop.WebhookEvent
	if err := json.Unmarshal(bodyBytes, &event); err != nil {
		log.Errorf("Failed to decode Whoop webhook: %s", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Respond quickly, process asynchronously
	go processWhoopWebhookEvent(event)

	w.WriteHeader(http.StatusOK)
}

func processWhoopWebhookEvent(event whoop.WebhookEvent) {
	switch event.Type {
	case "workout.updated":
		processWhoopWorkoutWebhook(event)
	case "workout.deleted":
		log.Infof("Whoop workout deleted: %s for user %d", event.ID, event.UserID)
		// Optionally handle deletion in the future
	default:
		log.Debugf("Ignoring Whoop webhook event type: %s", event.Type)
	}
}

func processWhoopWorkoutWebhook(event whoop.WebhookEvent) {
	if GlobalBot == nil {
		log.Error("GlobalBot not initialized, cannot process Whoop webhook")
		return
	}

	// Look up user by Whoop user ID
	user, err := users.GetUserByWhoopUserID(event.UserID)
	if err != nil {
		log.Errorf("No user found for Whoop user ID %d: %s", event.UserID, err)
		return
	}

	accessToken, err := user.GetValidWhoopAccessToken()
	if err != nil {
		log.Errorf("Failed to get token for user %s: %s", user.GetName(), err)
		return
	}

	record, err := whoop.GetWorkoutById(accessToken, event.ID)
	if err != nil {
		log.Errorf("Failed to fetch Whoop workout %s for user %s: %s", event.ID, user.GetName(), err)
		return
	}

	// If the workout already exists, this is an update (e.g., strain adjusted via Strength Trainer)
	// Edit the existing group notification messages with updated data
	if users.WorkoutExists(record.ID) {
		handleWhoopWorkoutUpdate(user, record)
		return
	}

	// Check if ignored
	if _, err := state.Get("whoop:ignored:" + record.ID); err == nil {
		return
	}

	// Check if pending user confirmation
	if _, err := state.Get("whoop:pending:" + record.ID); err == nil {
		return
	}

	// Check for duplicate image workout (time overlap)
	margin := 60 * time.Minute
	existing, err := user.GetWorkoutInTimeRange(record.Start.Add(-margin), record.End.Add(margin))
	if err == nil && existing.ID != 0 && existing.WhoopID == "" {
		log.Infof("Whoop webhook: linking workout %s to existing workout %d for user %s", record.ID, existing.ID, user.GetName())
		existing.WhoopID = record.ID
		db.DBCon.Save(&existing)
		return
	}

	// Filter: Ignore short workouts (< 25 mins) or low strain (< 4.0)
	duration := record.End.Sub(record.Start)
	if duration < 25*time.Minute || record.Score.Strain < 4.0 {
		return
	}

	if err := user.LoadGroups(); err != nil {
		log.Errorf("Failed to load groups for user %s: %s", user.GetName(), err)
		return
	}

	lastWorkout, err := user.GetLastWorkout()
	isBonus := err == nil && users.IsSameDay(lastWorkout.CreatedAt, record.Start)
	isSmall := record.Score.Strain < 10.0

	// Bonus or small workout: ask the user
	if isBonus || isSmall {
		msg := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf("I detected a workout: %s. Should this count as a workout?", record.SportName))
		yesBtn := tgbotapi.NewInlineKeyboardButtonData("Yes", fmt.Sprintf("whoop:yes:%s", record.ID))
		noBtn := tgbotapi.NewInlineKeyboardButtonData("No", fmt.Sprintf("whoop:no:%s", record.ID))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(yesBtn, noBtn))
		GlobalBot.Send(msg)

		state.SetWithTTL("whoop:pending:"+record.ID, "1", 86400) // 24h
		return
	}

	// Main workout: create and notify
	for _, group := range user.Groups {
		workout := users.Workout{
			UserID:  user.ID,
			GroupID: group.ID,
			WhoopID: record.ID,
		}
		db.DBCon.Create(&workout)
		notify.NotifyWorkout(GlobalBot, user, workout, record.SportName, record.Score.Strain, record.Score.Kilojoule/4.184, record.Score.AverageHeartRate, duration.Minutes(), 0, "", "")
	}
	notify.SendWorkoutPM(GlobalBot, user, record.SportName)
}

// handleWhoopWorkoutUpdate edits existing group notification messages when a workout is updated
// (e.g., user adjusted strain via Strength Trainer in the Whoop app)
func handleWhoopWorkoutUpdate(user users.User, record *whoop.WorkoutData) {
	workouts, err := users.GetWorkoutsByWhoopID(record.ID)
	if err != nil {
		log.Errorf("Failed to get workouts for Whoop ID %s: %s", record.ID, err)
		return
	}

	for _, workout := range workouts {
		if workout.NotifyMessageID == 0 || workout.NotifyChatID == 0 {
			continue
		}
		notify.EditWhoopNotification(GlobalBot, user, workout, record)
	}
	log.Infof("Whoop workout %s updated for user %s (strain: %.1f)", record.ID, user.GetName(), record.Score.Strain)
}
