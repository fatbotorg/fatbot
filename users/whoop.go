package users

import (
	"fatbot/db"
	"fatbot/whoop"
	"strconv"
	"time"
)

func (user *User) UpdateWhoopToken(token *whoop.TokenResponse) error {
	db := db.DBCon
	user.WhoopAccessToken = token.AccessToken
	user.WhoopRefreshToken = token.RefreshToken
	user.WhoopTokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	// Clear other integrations (one integration at a time rule)
	user.clearStrava()
	user.clearGarmin()

	return db.Save(&user).Error
}

func (user *User) GetValidWhoopAccessToken() (string, error) {
	if time.Now().After(user.WhoopTokenExpiry) {
		token, err := whoop.RefreshToken(user.WhoopRefreshToken)
		if err != nil {
			return "", err
		}
		if err := user.UpdateWhoopToken(token); err != nil {
			return "", err
		}
		return token.AccessToken, nil
	}
	return user.WhoopAccessToken, nil
}

func GetUserByState(state string) (User, error) {
	userId, err := strconv.ParseInt(state, 10, 64)
	if err != nil {
		return User{}, err
	}
	return GetUserById(userId)
}

func GetWhoopUsers() []User {
	db := db.DBCon
	var users []User
	db.Where("whoop_access_token != ?", "").Find(&users)
	return users
}

func GetUserByWhoopUserID(whoopUserID int64) (User, error) {
	var user User
	result := db.DBCon.Where("whoop_user_id = ?", whoopUserID).First(&user)
	return user, result.Error
}

func WorkoutExists(whoopID string) bool {
	db := db.DBCon
	var workout Workout
	db.Where("whoop_id = ?", whoopID).Limit(1).Find(&workout)
	return workout.ID != 0
}

func GetWorkoutsByWhoopID(whoopID string) ([]Workout, error) {
	var workouts []Workout
	result := db.DBCon.Where("whoop_id = ?", whoopID).Find(&workouts)
	return workouts, result.Error
}
