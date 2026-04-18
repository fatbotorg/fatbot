package updates

import (
	"encoding/json"
	"fatbot/db"
	"fatbot/garmin"
	"fatbot/schedule"
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleGarminCommand(fatBotUpdate FatBotUpdate) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.FromChat().ID, "")
	user, err := users.GetUserById(fatBotUpdate.Update.SentFrom().ID)
	if err != nil {
		return msg, err
	}

	stateStr := fmt.Sprintf("fatbot-%d-%d", user.TelegramUserID, time.Now().Unix())
	verifier := garmin.GenerateCodeVerifier()
	challenge := garmin.GenerateCodeChallenge(verifier)

	if err := state.SetWithTTL("garmin:verifier:"+stateStr, verifier, 600); err != nil {
		return msg, err
	}

	authURL := garmin.GetAuthURL(stateStr, challenge)
	log.Infof("Garmin Auth URL: %s", authURL)
	msg.Text = "Connect your Garmin account to automatically sync workouts."
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Connect Garmin", authURL),
		),
	)
	return msg, nil
}

func HandleGarminCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	stateStr := r.URL.Query().Get("state")

	if code == "" || stateStr == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	verifier, err := state.Get("garmin:verifier:" + stateStr)
	if err != nil {
		log.Errorf("Failed to retrieve Garmin verifier: %s", err)
		http.Error(w, "Expired or invalid state", http.StatusBadRequest)
		return
	}
	state.ClearString("garmin:verifier:" + stateStr)

	token, err := garmin.ExchangeToken(code, verifier)
	if err != nil {
		log.Errorf("Failed to exchange token: %s", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	parts := strings.Split(stateStr, "-")
	user, err := users.GetUserByState(parts[1])
	if err != nil {
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	// Fetch the real permanent User ID from Garmin's identity API
	garminID, err := garmin.GetUserID(token.AccessToken)
	if err == nil && garminID != "" {
		user.GarminUserID = garminID
		log.Infof("Registered Garmin User ID: %s for user %s", user.GarminUserID, user.GetName())
	} else {
		log.Errorf("Failed to fetch Garmin User ID: %v", err)
	}

	if err := user.UpdateGarminToken(token); err != nil {
		http.Error(w, "Failed to save token", http.StatusInternalServerError)
		return
	}

	log.Infof("Garmin connected for user %s, Garmin ID: %s", user.GetName(), user.GarminUserID)

	// Notify user via Telegram
	if GlobalBot != nil {
		msg := tgbotapi.NewMessage(user.TelegramUserID,
			"Your Garmin account has been connected successfully!\n\n"+
				"Your workouts will now be automatically synced. "+
				"Note: Any previous Strava or Whoop integration has been disconnected.")
		GlobalBot.Send(msg)
	}

	fmt.Fprint(w, "Garmin connected successfully! You can close this window.")
}

type GarminNotification struct {
	Activities      []GarminEntry `json:"activities"`
	Dailies         []GarminEntry `json:"dailies"`
	Epochs          []GarminEntry `json:"epochs"`
	Sleeps          []GarminEntry `json:"sleeps"`
	BodyComps       []GarminEntry `json:"bodyCompositions"`
	Stress          []GarminEntry `json:"stressDetails"`
	UserMetrics     []GarminEntry `json:"userMetrics"`
	MoveIQ          []GarminEntry `json:"moveIQActivities"`
	PulseOx         []GarminEntry `json:"pulseOx"`
	Respiration     []GarminEntry `json:"respiration"`
	ActivityDetails []GarminEntry `json:"activityDetails"`
	ActivityFiles   []GarminEntry `json:"activityFiles"`
}

type GarminEntry struct {
	UserId          string `json:"userId"`
	UserAccessToken string `json:"userAccessToken"`
	SummaryId       string `json:"summaryId"`
	CallbackURL     string `json:"callbackURL"`
	Summary         *struct {
		SummaryId          string  `json:"summaryId"`
		ActivityName       string  `json:"activityName"`
		ActivityType       string  `json:"activityType"`
		DeviceName         string  `json:"deviceName"`
		DurationInSeconds  int     `json:"durationInSeconds"`
		StartTimeInSeconds int64   `json:"startTimeInSeconds"`
		ActiveCalories     float64 `json:"activeKilocalories"`
		AverageHeartRate   int     `json:"averageHeartRateInBeatsPerMinute"`
		DistanceInMeters   float64 `json:"distanceInMeters"`
	} `json:"summary"`
	ActivityName       string  `json:"activityName"`
	ActivityType       string  `json:"activityType"`
	DeviceName         string  `json:"deviceName"`
	DurationInSeconds  int     `json:"durationInSeconds"`
	StartTimeInSeconds int64   `json:"startTimeInSeconds"`
	ActiveCalories     float64 `json:"activeCalories"`
	ActiveKilocalories float64 `json:"activeKilocalories"`
	Calories           float64 `json:"calories"`
	AverageHeartRate   int     `json:"averageHeartRateInBeatsPerMinute"`
	DistanceInMeters   float64 `json:"distanceInMeters"`
}

func (e GarminEntry) GetCalories() float64 {
	if e.ActiveCalories > 0 {
		return e.ActiveCalories
	}
	if e.ActiveKilocalories > 0 {
		return e.ActiveKilocalories
	}
	if e.Calories > 0 {
		return e.Calories
	}
	if e.Summary != nil && e.Summary.ActiveCalories > 0 {
		return e.Summary.ActiveCalories
	}
	return 0
}

const maxGarminActivitiesPerWebhookUser = 5

func findGarminWebhookUser(entry GarminEntry) (users.User, error) {
	var user users.User
	db.DBCon.Where("garmin_user_id = ?", entry.UserId).First(&user)
	if user.ID == 0 && entry.UserAccessToken != "" {
		db.DBCon.Where("garmin_access_token = ?", entry.UserAccessToken).First(&user)
		if user.ID != 0 && entry.UserId != "" {
			user.GarminUserID = entry.UserId
			db.DBCon.Save(&user)
		}
	}
	if user.ID == 0 {
		return user, fmt.Errorf("could not find Garmin user for userId=%s", entry.UserId)
	}
	return user, nil
}

func activitiesFromGarminEntry(entry GarminEntry, accessToken string) ([]garmin.ActivityData, error) {
	baseID := garmin.NormalizeSummaryID(entry.SummaryId)

	if entry.ActivityName != "" {
		if baseID == "" {
			return nil, fmt.Errorf("activity payload missing summaryId")
		}
		return []garmin.ActivityData{{
			SummaryID:          baseID,
			ActivityName:       entry.ActivityName,
			ActivityType:       entry.ActivityType,
			DeviceName:         entry.DeviceName,
			DurationInSeconds:  entry.DurationInSeconds,
			StartTimeInSeconds: entry.StartTimeInSeconds,
			Calories:           entry.GetCalories(),
			AverageHeartRate:   entry.AverageHeartRate,
			DistanceInMeters:   entry.DistanceInMeters,
		}}, nil
	}

	if entry.Summary != nil {
		baseID = garmin.NormalizeSummaryID(entry.Summary.SummaryId)
		if baseID == "" {
			return nil, fmt.Errorf("activity summary payload missing summaryId")
		}
		return []garmin.ActivityData{{
			SummaryID:          baseID,
			ActivityName:       entry.Summary.ActivityName,
			ActivityType:       entry.Summary.ActivityType,
			DeviceName:         entry.Summary.DeviceName,
			DurationInSeconds:  entry.Summary.DurationInSeconds,
			StartTimeInSeconds: entry.Summary.StartTimeInSeconds,
			Calories:           entry.GetCalories(),
			AverageHeartRate:   entry.Summary.AverageHeartRate,
			DistanceInMeters:   entry.Summary.DistanceInMeters,
		}}, nil
	}

	if entry.CallbackURL == "" || strings.Contains(entry.CallbackURL, "activityFile") {
		return nil, nil
	}

	fetched, err := garmin.FetchActivityByPullURI(entry.CallbackURL, accessToken)
	if err != nil {
		return nil, err
	}
	for i := range fetched {
		fetched[i].SummaryID = garmin.NormalizeSummaryID(fetched[i].SummaryID)
	}
	return fetched, nil
}

func limitGarminActivities(activities []garmin.ActivityData, max int) []garmin.ActivityData {
	if len(activities) <= max {
		return activities
	}
	limited := append([]garmin.ActivityData(nil), activities...)
	sort.Slice(limited, func(i, j int) bool {
		return limited[i].StartTimeInSeconds > limited[j].StartTimeInSeconds
	})
	return limited[:max]
}

func HandleGarminWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read Garmin webhook body: %s", err)
		return
	}
	log.Infof("Raw Garmin Webhook Body: %s", string(bodyBytes))

	var notification GarminNotification
	if err := json.Unmarshal(bodyBytes, &notification); err != nil {
		log.Errorf("Failed to decode Garmin webhook: %s", err)
		return
	}

	log.Infof(
		"Received Garmin Webhook: activities=%d activityDetails=%d ignored_non_activity=%d",
		len(notification.Activities),
		len(notification.ActivityDetails),
		len(notification.Dailies)+len(notification.Epochs)+len(notification.Sleeps)+len(notification.BodyComps)+len(notification.Stress)+len(notification.UserMetrics)+len(notification.MoveIQ)+len(notification.PulseOx)+len(notification.Respiration)+len(notification.ActivityFiles),
	)

	type webhookUserBatch struct {
		user       users.User
		activities []garmin.ActivityData
		seen       map[string]bool
	}

	batches := make(map[uint]*webhookUserBatch)
	for _, entry := range notification.Activities {
		user, err := findGarminWebhookUser(entry)
		if err != nil {
			log.Warnf("Skipping Garmin activity entry: %s", err)
			continue
		}

		accessToken, err := user.GetValidGarminAccessToken()
		if err != nil {
			log.Warnf("Skipping Garmin activity entry for user %s: %s", user.GetName(), err)
			continue
		}

		activities, err := activitiesFromGarminEntry(entry, accessToken)
		if err != nil {
			log.Warnf("Skipping Garmin activity payload for user %s: %s", user.GetName(), err)
			continue
		}

		batch := batches[user.ID]
		if batch == nil {
			batch = &webhookUserBatch{user: user, seen: make(map[string]bool)}
			batches[user.ID] = batch
		}

		for _, activity := range activities {
			activity.SummaryID = garmin.NormalizeSummaryID(activity.SummaryID)
			if activity.SummaryID == "" {
				log.Warnf("Skipping Garmin activity with empty summaryId for user %s", user.GetName())
				continue
			}
			if batch.seen[activity.SummaryID] {
				continue
			}
			batch.seen[activity.SummaryID] = true
			batch.activities = append(batch.activities, activity)
		}
	}

	for _, batch := range batches {
		if len(batch.activities) == 0 {
			continue
		}

		if len(batch.activities) > maxGarminActivitiesPerWebhookUser {
			log.Warnf(
				"Suspicious Garmin batch for user %s: %d activities in a single webhook, limiting to the most recent 1",
				batch.user.GetName(),
				len(batch.activities),
			)
			batch.activities = limitGarminActivities(batch.activities, 1)
			if GlobalBot != nil {
				msg := tgbotapi.NewMessage(
					batch.user.TelegramUserID,
					"I detected an unusually large Garmin sync and only kept the newest activity to prevent accidental bulk workout uploads.",
				)
				GlobalBot.Send(msg)
			}
		}

		for _, activity := range batch.activities {
			if GlobalBot != nil {
				schedule.ProcessGarminActivity(GlobalBot, batch.user, activity)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

type GarminPermissionsChange struct {
	PermissionsChange []struct {
		UserId          string   `json:"userId"`
		UserAccessToken string   `json:"userAccessToken"`
		Permissions     []string `json:"permissions"`
	} `json:"permissionsChange"`
}

func HandleGarminPermissionsChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read Garmin permissions change body: %s", err)
		return
	}
	log.Infof("Raw Garmin Permissions Change Body: %s", string(bodyBytes))

	var change GarminPermissionsChange
	if err := json.Unmarshal(bodyBytes, &change); err != nil {
		log.Errorf("Failed to decode Garmin permissions change: %s", err)
		return
	}

	for _, entry := range change.PermissionsChange {
		var user users.User
		db.DBCon.Where("garmin_user_id = ?", entry.UserId).First(&user)
		if user.ID == 0 && entry.UserAccessToken != "" {
			db.DBCon.Where("garmin_access_token = ?", entry.UserAccessToken).First(&user)
		}

		if user.ID != 0 {
			if len(entry.Permissions) == 0 {
				if err := user.DeregisterGarmin(); err != nil {
					log.Errorf("Failed to deregister Garmin for user %s after permission revocation: %s", user.GetName(), err)
				} else {
					log.Infof("Successfully deregistered Garmin for user %s due to full permission revocation", user.GetName())
					if GlobalBot != nil {
						msg := tgbotapi.NewMessage(user.TelegramUserID, "Your Garmin account has been disconnected because all permissions were revoked.")
						GlobalBot.Send(msg)
					}
				}
			} else {
				log.Infof("Garmin permissions changed for user %s: %v", user.GetName(), entry.Permissions)
			}
		} else {
			log.Warnf("Could not find user for permissions change: UserId=%s", entry.UserId)
		}
	}

	w.WriteHeader(http.StatusOK)
}

var GlobalBot *tgbotapi.BotAPI

type GarminDeregistration struct {
	Deregistrations []struct {
		UserId          string `json:"userId"`
		UserAccessToken string `json:"userAccessToken"`
	} `json:"deregistrations"`
}

func HandleGarminDeregistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read Garmin deregistration body: %s", err)
		return
	}
	log.Infof("Raw Garmin Deregistration Body: %s", string(bodyBytes))

	var dereg GarminDeregistration
	if err := json.Unmarshal(bodyBytes, &dereg); err != nil {
		log.Errorf("Failed to decode Garmin deregistration: %s", err)
		return
	}

	for _, entry := range dereg.Deregistrations {
		var user users.User
		db.DBCon.Where("garmin_user_id = ?", entry.UserId).First(&user)
		if user.ID == 0 && entry.UserAccessToken != "" {
			db.DBCon.Where("garmin_access_token = ?", entry.UserAccessToken).First(&user)
		}

		if user.ID != 0 {
			if err := user.DeregisterGarmin(); err != nil {
				log.Errorf("Failed to deregister Garmin for user %s: %s", user.GetName(), err)
			} else {
				log.Infof("Successfully deregistered Garmin for user %s", user.GetName())
				if GlobalBot != nil {
					msg := tgbotapi.NewMessage(user.TelegramUserID, "Your Garmin account has been disconnected.")
					GlobalBot.Send(msg)
				}
			}
		} else {
			log.Warnf("Could not find user to deregister: UserId=%s", entry.UserId)
		}
	}

	w.WriteHeader(http.StatusOK)
}
