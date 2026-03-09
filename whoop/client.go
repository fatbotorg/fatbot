package whoop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

const (
	AuthURL    = "https://api.prod.whoop.com/oauth/oauth2/auth"
	TokenURL   = "https://api.prod.whoop.com/oauth/oauth2/token"
	WorkoutURL = "https://api.prod.whoop.com/developer/v2/activity/workout"
	CycleURL   = "https://api.prod.whoop.com/developer/v1/cycle"
	ProfileURL = "https://api.prod.whoop.com/developer/v2/user/profile/basic"
	Scope      = "read:workout read:cycles read:profile offline"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

type CycleResponse struct {
	Records []CycleData `json:"records"`
}

type CycleData struct {
	ID    int        `json:"id"`
	Score CycleScore `json:"score"`
	Start time.Time  `json:"start"`
	End   time.Time  `json:"end"`
}

type CycleScore struct {
	Strain           float64 `json:"strain"`
	Kilojoule        float64 `json:"kilojoule"`
	AverageHeartRate int     `json:"average_heart_rate"`
	MaxHeartRate     int     `json:"max_heart_rate"`
}

type WorkoutResponse struct {
	Records   []WorkoutData `json:"records"`
	NextToken string        `json:"next_token"`
}

type WorkoutScore struct {
	Strain           float64 `json:"strain"`
	Kilojoule        float64 `json:"kilojoule"`
	AverageHeartRate int     `json:"average_heart_rate"`
	MaxHeartRate     int     `json:"max_heart_rate"`
}

type WorkoutData struct {
	ID         string       `json:"id"`
	SportID    int          `json:"sport_id"`
	SportName  string       `json:"sport_name"`
	ScoreState string       `json:"score_state"` // "SCORED", "PENDING_SCORE", "UNSCORABLE"
	Score      WorkoutScore `json:"score"`
	Start      time.Time    `json:"start"`
	End        time.Time    `json:"end"`
}

// WebhookEvent represents a Whoop webhook payload
type WebhookEvent struct {
	UserID  int64  `json:"user_id"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	TraceID string `json:"trace_id"`
}

// UserProfile represents the Whoop user profile response
type UserProfile struct {
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func GetAuthURL(state string) string {
	clientID := viper.GetString("whoop.client_id")
	redirectURI := viper.GetString("whoop.redirect_uri")

	u, _ := url.Parse(AuthURL)
	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", Scope)
	q.Set("state", state)
	u.RawQuery = q.Encode()

	return u.String()
}

func ExchangeToken(code string) (*TokenResponse, error) {
	clientID := viper.GetString("whoop.client_id")
	clientSecret := viper.GetString("whoop.client_secret")
	redirectURI := viper.GetString("whoop.redirect_uri")

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)

	resp, err := http.PostForm(TokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to exchange token: %s", resp.Status)
	}

	var tokenResponse TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, err
	}
	return &tokenResponse, nil
}

func RefreshToken(refreshToken string) (*TokenResponse, error) {
	clientID := viper.GetString("whoop.client_id")
	clientSecret := viper.GetString("whoop.client_secret")

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("scope", Scope)

	resp, err := http.PostForm(TokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to refresh token: %s", resp.Status)
	}

	var tokenResponse TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, err
	}
	return &tokenResponse, nil
}

func GetWorkouts(accessToken string, start time.Time, nextToken string) (*WorkoutResponse, error) {
	u, _ := url.Parse(WorkoutURL)
	q := u.Query()
	q.Set("start", start.Format(time.RFC3339))
	if nextToken != "" {
		q.Set("nextToken", nextToken)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
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
		log.Errorf("Whoop API Error: %s Body: %s", resp.Status, string(body))
		return nil, fmt.Errorf("failed to fetch workouts: %s", resp.Status)
	}

	var workoutResponse WorkoutResponse
	if err := json.NewDecoder(resp.Body).Decode(&workoutResponse); err != nil {
		return nil, err
	}
	return &workoutResponse, nil
}

func GetCycleCollection(accessToken string, start time.Time) (*CycleResponse, error) {
	u, _ := url.Parse(CycleURL)
	q := u.Query()
	q.Set("start", start.Format(time.RFC3339))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
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
		log.Errorf("Whoop API Error (Cycle): %s Body: %s", resp.Status, string(body))
		return nil, fmt.Errorf("failed to fetch cycles: %s", resp.Status)
	}

	var cycleResponse CycleResponse
	if err := json.NewDecoder(resp.Body).Decode(&cycleResponse); err != nil {
		return nil, err
	}
	return &cycleResponse, nil
}

func GetWorkoutById(accessToken string, workoutID string) (*WorkoutData, error) {
	u, _ := url.Parse(WorkoutURL + "/" + workoutID)

	req, err := http.NewRequest("GET", u.String(), nil)
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
		log.Errorf("Whoop API Error (GetById): %s Body: %s", resp.Status, string(body))
		return nil, fmt.Errorf("failed to fetch workout: %s", resp.Status)
	}

	var workoutData WorkoutData
	if err := json.NewDecoder(resp.Body).Decode(&workoutData); err != nil {
		return nil, err
	}
	return &workoutData, nil
}

// GetUserProfile fetches the authenticated user's basic profile (user_id, name, email)
func GetUserProfile(accessToken string) (*UserProfile, error) {
	req, err := http.NewRequest("GET", ProfileURL, nil)
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
		log.Errorf("Whoop API Error (Profile): %s Body: %s", resp.Status, string(body))
		return nil, fmt.Errorf("failed to fetch user profile: %s", resp.Status)
	}

	var profile UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// ValidateWebhookSignature verifies a Whoop webhook signature.
// The signature is computed as: base64(HMAC-SHA256(timestamp + body, clientSecret))
func ValidateWebhookSignature(body []byte, timestamp, signature, clientSecret string) bool {
	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write([]byte(timestamp))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
