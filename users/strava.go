package users

import (
	"fatbot/db"
	"fatbot/strava"
	"fmt"
	"time"
)

// UpdateStravaToken saves the Strava tokens to the user and clears other integrations
func (user *User) UpdateStravaToken(token *strava.TokenResponse) error {
	db := db.DBCon
	user.StravaAccessToken = token.AccessToken
	user.StravaRefreshToken = token.RefreshToken
	// Strava returns expires_at as unix timestamp
	user.StravaTokenExpiry = time.Unix(token.ExpiresAt, 0)

	// Store the athlete ID if provided
	if token.Athlete != nil {
		user.StravaAthleteID = fmt.Sprintf("%d", token.Athlete.ID)
	}

	// Clear other integrations (one integration at a time rule)
	user.ClearOtherIntegrations()

	return db.Save(&user).Error
}

// GetValidStravaAccessToken returns a valid access token, refreshing if expired
func (user *User) GetValidStravaAccessToken() (string, error) {
	if time.Now().After(user.StravaTokenExpiry) {
		token, err := strava.RefreshToken(user.StravaRefreshToken)
		if err != nil {
			return "", err
		}
		if err := user.UpdateStravaToken(token); err != nil {
			return "", err
		}
		return token.AccessToken, nil
	}
	return user.StravaAccessToken, nil
}

// clearWhoop clears Whoop tokens
func (user *User) clearWhoop() {
	user.WhoopAccessToken = ""
	user.WhoopRefreshToken = ""
	user.WhoopTokenExpiry = time.Time{}
	user.WhoopID = ""
}

// clearGarmin clears Garmin tokens
func (user *User) clearGarmin() {
	user.GarminAccessToken = ""
	user.GarminRefreshToken = ""
	user.GarminTokenExpiry = time.Time{}
	user.GarminUserID = ""
}

// clearStrava clears Strava tokens
func (user *User) clearStrava() {
	user.StravaAccessToken = ""
	user.StravaRefreshToken = ""
	user.StravaTokenExpiry = time.Time{}
	user.StravaAthleteID = ""
}

// ClearOtherIntegrations clears Whoop and Garmin tokens when Strava is connected
func (user *User) ClearOtherIntegrations() {
	user.clearWhoop()
	user.clearGarmin()
}

// DeregisterStrava clears all Strava tokens and athlete ID
func (user *User) DeregisterStrava() error {
	db := db.DBCon
	user.StravaAccessToken = ""
	user.StravaRefreshToken = ""
	user.StravaTokenExpiry = time.Time{}
	user.StravaAthleteID = ""
	return db.Save(&user).Error
}

// GetStravaUsers returns all users with a Strava access token
func GetStravaUsers() []User {
	db := db.DBCon
	var users []User
	db.Where("strava_access_token != ?", "").Find(&users)
	return users
}

// GetUserByStravaAthleteID finds a user by their Strava athlete ID
func GetUserByStravaAthleteID(athleteID string) (User, error) {
	db := db.DBCon
	var user User
	if err := db.Where("strava_athlete_id = ?", athleteID).First(&user).Error; err != nil {
		return user, err
	}
	if user.ID == 0 {
		return user, fmt.Errorf("no user found with Strava athlete ID: %s", athleteID)
	}
	return user, nil
}

// StravaWorkoutExists checks if a workout with the given Strava ID already exists
func StravaWorkoutExists(stravaID string) bool {
	db := db.DBCon
	var workout Workout
	db.Where("strava_id = ?", stravaID).Limit(1).Find(&workout)
	return workout.ID != 0
}
