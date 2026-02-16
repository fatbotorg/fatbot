package garmin

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type ActivityData struct {
	SummaryID          string  `json:"summaryId"`
	ActivityID         int64   `json:"activityId"`
	ActivityName       string  `json:"activityName"`
	ActivityType       string  `json:"activityType"`
	DeviceName         string  `json:"deviceName"`
	StartTimeInSeconds int64   `json:"startTimeInSeconds"`
	DurationInSeconds  int     `json:"durationInSeconds"`
	Calories           float64 `json:"calories"`
	DistanceInMeters   float64 `json:"distanceInMeters"`
	AverageHeartRate   int     `json:"averageHeartRateInBeatsPerMinute"`
}

func getBaseAuthURL() string {
	return "https://connect.garmin.com"
}

func getBaseTokenURL() string {
	return "https://diauth.garmin.com"
}

func getBaseAPIURL() string {
	// For Health API Pull requests, this host is mandatory.
	return "https://healthapi.garmin.com"
}

func GenerateCodeVerifier() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	b := make([]byte, 64)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func GenerateCodeChallenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func GetAuthURL(state, challenge string) string {
	clientID := viper.GetString("garmin.client_id")
	if clientID == "" {
		log.Error("GARMIN_CLIENT_ID is not set!")
	}
	redirectURI := viper.GetString("garmin.redirect_uri")
	env := viper.GetString("garmin.environment")
	if env == "" {
		env = "sandbox"
	}

	authURL := getBaseAuthURL() + "/oauth2Confirm"
	u, _ := url.Parse(authURL)
	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("scope", "read:activities read:activity_data read:daily_summaries read:historical_data read:sleep read:body_composition read:stress read:user_metrics read:move_iq read:pulse_ox read:respiration")
	u.RawQuery = q.Encode()

	log.Infof("Generating Garmin Auth URL for environment: %s", env)
	return u.String()
}

func ExchangeToken(code, verifier string) (*TokenResponse, error) {
	clientID := viper.GetString("garmin.client_id")
	clientSecret := viper.GetString("garmin.client_secret")
	redirectURI := viper.GetString("garmin.redirect_uri")

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code_verifier", verifier)
	data.Set("redirect_uri", redirectURI)

	tokenURL := getBaseTokenURL() + "/di-oauth2-service/oauth/token"
	resp, err := http.PostForm(tokenURL, data)
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

func RefreshToken(refreshToken string) (*TokenResponse, error) {
	clientID := viper.GetString("garmin.client_id")
	clientSecret := viper.GetString("garmin.client_secret")

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	tokenURL := getBaseTokenURL() + "/di-oauth2-service/oauth/token"
	resp, err := http.PostForm(tokenURL, data)
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

func GetActivities(accessToken string, start time.Time) ([]ActivityData, error) {
	// Webhooks are mandatory for Health partners. Polling always returns 400.
	return nil, nil
}

func FetchActivityByPullURI(pullURI, accessToken string) ([]ActivityData, error) {
	req, err := http.NewRequest("GET", pullURI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch pull URI: %s, body: %s", resp.Status, string(body))
	}

	var activities []ActivityData
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		return nil, err
	}
	return activities, nil
}

func GetUserID(accessToken string) (string, error) {
	// Identity endpoint to get the real permanent User ID used in webhooks
	url := "https://healthapi.garmin.com/wellness-api/rest/user/id"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get user ID: %s", resp.Status)
	}

	var result struct {
		UserId string `json:"userId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.UserId, nil
}
