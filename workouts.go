package main

import (
	"time"
)

// TODO:
// Remove last_last field and use workouts table
func rollbackLastWorkout(userId int64) (Workout, error) {
	db := getDB()
	var user User
	if err := db.Where(User{TelegramUserID: userId}).First(&user).Error; err != nil {
		return Workout{}, err
	} else {
		lastWorkout, err := getLastWorkout(userId)
		if err != nil {
			return Workout{}, err
		}
		db.Delete(&Workout{}, lastWorkout.ID)
	}
	newLastWorkout, err := getLastWorkout(userId)
	if err != nil {
		return Workout{}, err
	}
	return newLastWorkout, nil
}

func getLastWorkout(userId int64) (Workout, error) {
	db := getDB()
	var user User
	if err := db.Model(&User{}).Where("telegram_user_id = ?", userId).Preload("Workouts").Limit(2).Find(&user).Error; err != nil {
		return Workout{}, err
	}

	if len(user.Workouts) == 0 {
		return Workout{}, nil
	}
	return user.Workouts[len(user.Workouts)-1], nil
}

func updateWorkout(userId int64, messageId int) error {
	db := getDB()
	var user User
	db.Where("telegram_user_id = ?", userId).Find(&user)
	when := time.Now()
	// TODO:
	// Allow old updates
	//
	// if daysago != 0 {
	// 	when = time.Now().Add(time.Duration(-24*daysago) * time.Hour)
	// }
	db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("last_workout", when)
	workout := &Workout{
		UserID:         user.ID,
		PhotoMessageID: messageId,
	}
	db.Model(&user).Association("Workouts").Append(workout)
	return nil
}
