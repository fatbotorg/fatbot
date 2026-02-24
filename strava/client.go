package strava

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

const (
	AuthURL  = "https://www.strava.com/oauth/authorize"
	TokenURL = "https://www.strava.com/api/v3/oauth/token"
	BaseAPI  = "https://www.strava.com/api/v3"
	// Scopes needed: activity:read_all for private activities and privacy webhooks
	Scope = "activity:read_all,read"
)

// TokenResponse represents the OAuth token response from Strava
type TokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    int64        `json:"expires_at"`
	ExpiresIn    int          `json:"expires_in"`
	TokenType    string       `json:"token_type"`
	Athlete      *AthleteData `json:"athlete,omitempty"`
}

// AthleteData represents the athlete info returned during OAuth
type AthleteData struct {
	ID            int64  `json:"id"`
	Username      string `json:"username"`
	FirstName     string `json:"firstname"`
	LastName      string `json:"lastname"`
	City          string `json:"city"`
	State         string `json:"state"`
	Country       string `json:"country"`
	ProfileMedium string `json:"profile_medium"`
	Profile       string `json:"profile"`
}

// ActivityData represents a Strava activity
type ActivityData struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	SportType          string  `json:"sport_type"`
	Distance           float64 `json:"distance"`     // meters
	MovingTime         int     `json:"moving_time"`  // seconds
	ElapsedTime        int     `json:"elapsed_time"` // seconds
	TotalElevationGain float64 `json:"total_elevation_gain"`
	StartDate          string  `json:"start_date"`
	StartDateLocal     string  `json:"start_date_local"`
	Timezone           string  `json:"timezone"`
	AverageSpeed       float64 `json:"average_speed"` // m/s
	MaxSpeed           float64 `json:"max_speed"`     // m/s
	AverageHeartrate   float64 `json:"average_heartrate"`
	MaxHeartrate       float64 `json:"max_heartrate"`
	Calories           float64 `json:"calories"`
	SufferScore        *int    `json:"suffer_score"` // Strava Premium feature, may be null
	DeviceName         string  `json:"device_name"`
	Manual             bool    `json:"manual"`
	Private            bool    `json:"private"`
	Commute            bool    `json:"commute"`
	Trainer            bool    `json:"trainer"`
}

// WebhookEvent represents a Strava webhook push event
type WebhookEvent struct {
	ObjectType     string            `json:"object_type"` // "activity" or "athlete"
	ObjectID       int64             `json:"object_id"`   // activity or athlete ID
	AspectType     string            `json:"aspect_type"` // "create", "update", "delete"
	Updates        map[string]string `json:"updates"`     // For update events
	OwnerID        int64             `json:"owner_id"`    // athlete ID
	SubscriptionID int               `json:"subscription_id"`
	EventTime      int64             `json:"event_time"`
}

// WebhookValidation represents the validation request from Strava
type WebhookValidation struct {
	HubMode      string `json:"hub.mode"`
	HubChallenge string `json:"hub.challenge"`
	HubVerify    string `json:"hub.verify_token"`
}

// GetAuthURL generates the OAuth authorization URL
func GetAuthURL(state string) string {
	clientID := viper.GetString("strava.client_id")
	redirectURI := viper.GetString("strava.redirect_uri")

	u, _ := url.Parse(AuthURL)
	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", Scope)
	q.Set("state", state)
	q.Set("approval_prompt", "auto")
	u.RawQuery = q.Encode()

	return u.String()
}

// ExchangeToken exchanges an authorization code for access/refresh tokens
func ExchangeToken(code string) (*TokenResponse, error) {
	clientID := viper.GetString("strava.client_id")
	clientSecret := viper.GetString("strava.client_secret")

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")

	resp, err := http.PostForm(TokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to exchange token: %s, body: %s", resp.Status, string(body))
	}

	var tokenResponse TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, err
	}
	return &tokenResponse, nil
}

// RefreshToken refreshes an expired access token
func RefreshToken(refreshToken string) (*TokenResponse, error) {
	clientID := viper.GetString("strava.client_id")
	clientSecret := viper.GetString("strava.client_secret")

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	resp, err := http.PostForm(TokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to refresh token: %s, body: %s", resp.Status, string(body))
	}

	var tokenResponse TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, err
	}
	return &tokenResponse, nil
}

// GetActivity fetches a specific activity by ID
func GetActivity(accessToken string, activityID int64) (*ActivityData, error) {
	url := fmt.Sprintf("%s/activities/%d", BaseAPI, activityID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Errorf("Strava API Error: %s Body: %s", resp.Status, string(body))
		return nil, fmt.Errorf("failed to fetch activity: %s", resp.Status)
	}

	var activity ActivityData
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		return nil, err
	}
	return &activity, nil
}

// GetAthlete fetches the authenticated athlete's profile
func GetAthlete(accessToken string) (*AthleteData, error) {
	url := fmt.Sprintf("%s/athlete", BaseAPI)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch athlete: %s, body: %s", resp.Status, string(body))
	}

	var athlete AthleteData
	if err := json.NewDecoder(resp.Body).Decode(&athlete); err != nil {
		return nil, err
	}
	return &athlete, nil
}

// SufferScoreToStrain converts Strava's Suffer Score to an approximate Whoop-like strain
// Suffer Score range: 0-400+ (typically 0-150 for most workouts)
// Whoop Strain range: 0-21
// This is a rough approximation for display comparison only
func SufferScoreToStrain(sufferScore int) float64 {
	// Linear approximation: strain = suffer_score * 0.14
	// This gives: suffer 50 -> strain 7, suffer 100 -> strain 14, suffer 150 -> strain 21
	strain := float64(sufferScore) * 0.14
	if strain > 21.0 {
		strain = 21.0
	}
	return strain
}
