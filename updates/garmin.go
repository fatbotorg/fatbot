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

	allEntries := append(notification.Activities, notification.Dailies...)
	allEntries = append(allEntries, notification.Epochs...)
	allEntries = append(allEntries, notification.Sleeps...)
	allEntries = append(allEntries, notification.BodyComps...)
	allEntries = append(allEntries, notification.Stress...)
	allEntries = append(allEntries, notification.UserMetrics...)
	allEntries = append(allEntries, notification.MoveIQ...)
	allEntries = append(allEntries, notification.PulseOx...)
	allEntries = append(allEntries, notification.Respiration...)
	allEntries = append(allEntries, notification.ActivityDetails...)

	log.Infof("Received Garmin Webhook: %d total entries", len(allEntries))

	processedIDs := make(map[string]bool)
	for _, entry := range allEntries {
		// Normalize ID: strip suffixes like -detail, -file, etc.
		baseID := entry.SummaryId
		if idx := strings.Index(baseID, "-"); idx != -1 {
			baseID = baseID[:idx]
		}

		if processedIDs[baseID] {
			continue
		}
		processedIDs[baseID] = true

		// Find user
		var user users.User
		db.DBCon.Where("garmin_user_id = ?", entry.UserId).First(&user)
		if user.ID == 0 && entry.UserAccessToken != "" {
			db.DBCon.Where("garmin_access_token = ?", entry.UserAccessToken).First(&user)
			if user.ID != 0 {
				user.GarminUserID = entry.UserId
				db.DBCon.Save(&user)
			}
		}

		if user.ID == 0 {
			continue
		}

		accessToken, err := user.GetValidGarminAccessToken()
		if err != nil {
			continue
		}

		var activities []garmin.ActivityData

		if entry.ActivityName != "" {
			activities = []garmin.ActivityData{{
				SummaryID:          baseID,
				ActivityName:       entry.ActivityName,
				ActivityType:       entry.ActivityType,
				DeviceName:         entry.DeviceName,
				DurationInSeconds:  entry.DurationInSeconds,
				StartTimeInSeconds: entry.StartTimeInSeconds,
				Calories:           entry.GetCalories(),
				AverageHeartRate:   entry.AverageHeartRate,
				DistanceInMeters:   entry.DistanceInMeters,
			}}
		} else if entry.Summary != nil {
			activities = []garmin.ActivityData{{
				SummaryID:          baseID,
				ActivityName:       entry.Summary.ActivityName,
				ActivityType:       entry.Summary.ActivityType,
				DeviceName:         entry.Summary.DeviceName,
				DurationInSeconds:  entry.Summary.DurationInSeconds,
				StartTimeInSeconds: entry.Summary.StartTimeInSeconds,
				Calories:           entry.GetCalories(),
				AverageHeartRate:   entry.Summary.AverageHeartRate,
				DistanceInMeters:   entry.Summary.DistanceInMeters,
			}}
		} else if entry.CallbackURL != "" && !strings.Contains(entry.CallbackURL, "activityFile") {
			fetched, err := garmin.FetchActivityByPullURI(entry.CallbackURL, accessToken)
			if err == nil {
				for i := range fetched {
					if idx := strings.Index(fetched[i].SummaryID, "-"); idx != -1 {
						fetched[i].SummaryID = fetched[i].SummaryID[:idx]
					}
				}
				activities = fetched
			}
		}

		for _, activity := range activities {
			if GlobalBot != nil {
				schedule.ProcessGarminActivity(GlobalBot, user, activity)
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
