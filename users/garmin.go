package users

import (
	"fatbot/db"
	"fatbot/garmin"
	"time"
)

func (user *User) UpdateGarminToken(token *garmin.TokenResponse) error {
	db := db.DBCon
	user.GarminAccessToken = token.AccessToken
	user.GarminRefreshToken = token.RefreshToken
	user.GarminTokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	// Clear other integrations (one integration at a time rule)
	user.clearStrava()
	user.clearWhoop()

	return db.Save(&user).Error
}

func (user *User) GetValidGarminAccessToken() (string, error) {
	if time.Now().After(user.GarminTokenExpiry) {
		token, err := garmin.RefreshToken(user.GarminRefreshToken)
		if err != nil {
			return "", err
		}
		if err := user.UpdateGarminToken(token); err != nil {
			return "", err
		}
		return token.AccessToken, nil
	}
	return user.GarminAccessToken, nil
}

func GetGarminUsers() []User {
	db := db.DBCon
	var users []User
	db.Where("garmin_access_token != ?", "").Find(&users)
	return users
}

func GarminWorkoutExists(garminID string) bool {
	baseID := garmin.NormalizeSummaryID(garminID)

	db := db.DBCon
	var workout Workout
	db.Where("garmin_id = ?", baseID).Limit(1).Find(&workout)
	return workout.ID != 0
}

func (user *User) DeregisterGarmin() error {
	db := db.DBCon
	user.GarminAccessToken = ""
	user.GarminRefreshToken = ""
	user.GarminTokenExpiry = time.Time{}
	user.GarminUserID = ""
	return db.Save(&user).Error
}
