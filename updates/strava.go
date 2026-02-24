package updates

import (
	"encoding/json"
	"fatbot/schedule"
	"fatbot/strava"
	"fatbot/users"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

// HandleStravaCommand handles the /strava command to initiate OAuth flow
func HandleStravaCommand(fatBotUpdate FatBotUpdate) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.FromChat().ID, "")
	user, err := users.GetUserById(fatBotUpdate.Update.SentFrom().ID)
	if err != nil {
		return msg, err
	}

	// Generate state for OAuth (includes user ID for callback identification)
	state := fmt.Sprintf("fatbot-%d-%d", user.TelegramUserID, time.Now().Unix())
	authURL := strava.GetAuthURL(state)
	log.Infof("Strava Auth URL: %s", authURL)

	msg.Text = "Connect your Strava account to automatically sync workouts.\n\n" +
		"Note: This will disconnect any existing Whoop or Garmin integration."
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Connect Strava", authURL),
		),
	)
	return msg, nil
}

// HandleStravaCallback handles the OAuth callback from Strava
func HandleStravaCallback(w http.ResponseWriter, r *http.Request) {
	log.Infof("Strava Callback received: %s", r.URL.String())
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	// Handle denial
	if errorParam != "" {
		log.Warnf("Strava OAuth denied: %s", errorParam)
		http.Error(w, "Authorization was denied", http.StatusUnauthorized)
		return
	}

	if code == "" || state == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	token, err := strava.ExchangeToken(code)
	if err != nil {
		log.Errorf("Failed to exchange Strava token: %s", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	// Parse state to get user ID
	// State format: "fatbot-USERID-TIMESTAMP"
	parts := strings.Split(state, "-")
	if len(parts) < 2 {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	user, err := users.GetUserByState(parts[1])
	if err != nil {
		log.Errorf("Failed to find user from state: %s", err)
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	// Save tokens (this also clears Whoop/Garmin integrations)
	if err := user.UpdateStravaToken(token); err != nil {
		log.Errorf("Failed to save Strava token for user %s: %s", user.GetName(), err)
		http.Error(w, "Failed to save token", http.StatusInternalServerError)
		return
	}

	athleteName := ""
	if token.Athlete != nil {
		athleteName = fmt.Sprintf(" (%s %s)", token.Athlete.FirstName, token.Athlete.LastName)
	}

	log.Infof("Strava connected for user %s, athlete ID: %s%s", user.GetName(), user.StravaAthleteID, athleteName)

	// Notify user via Telegram
	if GlobalBot != nil {
		msg := tgbotapi.NewMessage(user.TelegramUserID,
			fmt.Sprintf("Your Strava account%s has been connected successfully!\n\n"+
				"Your workouts will now be automatically synced. "+
				"Note: Any previous Whoop or Garmin integration has been disconnected.", athleteName))
		GlobalBot.Send(msg)
	}

	fmt.Fprint(w, "Strava connected successfully! You can close this window.")
}

// HandleStravaWebhook handles both webhook validation (GET) and events (POST)
func HandleStravaWebhook(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleStravaWebhookValidation(w, r)
	case http.MethodPost:
		handleStravaWebhookEvent(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStravaWebhookValidation handles the initial webhook subscription validation
func handleStravaWebhookValidation(w http.ResponseWriter, r *http.Request) {
	hubMode := r.URL.Query().Get("hub.mode")
	hubChallenge := r.URL.Query().Get("hub.challenge")
	hubVerifyToken := r.URL.Query().Get("hub.verify_token")

	expectedVerifyToken := viper.GetString("strava.verify_token")

	log.Infof("Strava Webhook Validation: mode=%s, verify_token=%s", hubMode, hubVerifyToken)

	if hubMode != "subscribe" {
		http.Error(w, "Invalid hub.mode", http.StatusBadRequest)
		return
	}

	if hubVerifyToken != expectedVerifyToken {
		log.Errorf("Strava webhook verify token mismatch: got %s, expected %s", hubVerifyToken, expectedVerifyToken)
		http.Error(w, "Invalid verify token", http.StatusForbidden)
		return
	}

	// Respond with the challenge to confirm the webhook
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"hub.challenge": hubChallenge}
	json.NewEncoder(w).Encode(response)
	log.Info("Strava webhook validation successful")
}

// handleStravaWebhookEvent handles incoming webhook events
func handleStravaWebhookEvent(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read Strava webhook body: %s", err)
		w.WriteHeader(http.StatusOK) // Always respond 200 to Strava
		return
	}
	log.Infof("Raw Strava Webhook Body: %s", string(bodyBytes))

	var event strava.WebhookEvent
	if err := json.Unmarshal(bodyBytes, &event); err != nil {
		log.Errorf("Failed to decode Strava webhook: %s", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Process the event asynchronously to respond within 2 seconds
	go processStravaWebhookEvent(event)

	w.WriteHeader(http.StatusOK)
}

// processStravaWebhookEvent handles the webhook event processing
func processStravaWebhookEvent(event strava.WebhookEvent) {
	log.Infof("Processing Strava webhook event: type=%s, object=%s, aspect=%s, owner=%d, object_id=%d",
		event.ObjectType, event.AspectType, event.ObjectType, event.OwnerID, event.ObjectID)

	switch event.ObjectType {
	case "activity":
		handleStravaActivityEvent(event)
	case "athlete":
		handleStravaAthleteEvent(event)
	default:
		log.Warnf("Unknown Strava webhook object type: %s", event.ObjectType)
	}
}

// handleStravaActivityEvent handles activity-related webhook events
func handleStravaActivityEvent(event strava.WebhookEvent) {
	ownerID := fmt.Sprintf("%d", event.OwnerID)

	// Find the user by Strava athlete ID
	user, err := users.GetUserByStravaAthleteID(ownerID)
	if err != nil {
		log.Warnf("Could not find user for Strava athlete ID %s: %s", ownerID, err)
		return
	}

	switch event.AspectType {
	case "create":
		// New activity created - process it
		if GlobalBot != nil {
			schedule.ProcessStravaActivity(GlobalBot, user, event.ObjectID)
		}
	case "update":
		// Activity updated - we could handle privacy changes here
		// For now, we just log it
		log.Infof("Strava activity %d updated for user %s: %v", event.ObjectID, user.GetName(), event.Updates)
	case "delete":
		// Activity deleted - we could handle this, but for now just log
		log.Infof("Strava activity %d deleted for user %s", event.ObjectID, user.GetName())
	}
}

// handleStravaAthleteEvent handles athlete-related webhook events (e.g., deauthorization)
func handleStravaAthleteEvent(event strava.WebhookEvent) {
	ownerID := fmt.Sprintf("%d", event.OwnerID)

	user, err := users.GetUserByStravaAthleteID(ownerID)
	if err != nil {
		log.Warnf("Could not find user for Strava athlete ID %s: %s", ownerID, err)
		return
	}

	// Check for deauthorization
	if event.AspectType == "update" {
		if authorized, exists := event.Updates["authorized"]; exists && authorized == "false" {
			// User revoked access
			if err := user.DeregisterStrava(); err != nil {
				log.Errorf("Failed to deregister Strava for user %s: %s", user.GetName(), err)
				return
			}
			log.Infof("Strava deauthorized for user %s", user.GetName())

			// Notify user
			if GlobalBot != nil {
				msg := tgbotapi.NewMessage(user.TelegramUserID,
					"Your Strava account has been disconnected because you revoked access in Strava settings.")
				GlobalBot.Send(msg)
			}
		}
	}
}
