package main

import (
	"time"

	"github.com/charmbracelet/log"
)

func rollbackLastWorkout(user_id int64) error {
	db := getDB()
	var user User
	if err := db.Where(User{TelegramUserID: user_id}).First(&user).Error; err != nil {
		log.Error(err)
		return err
	} else {
		db.Model(&user).Where("telegram_user_id = ?", user_id).Update("last_workout", user.LastLastWorkout)
	}
	return nil
}

func updateWorkout(user_id int64, daysago int64) error {
	db := getDB()
	var user User
	db.Where("telegram_user_id = ?", user_id).Find(&user)
	db.Model(&user).Where("telegram_user_id = ?", user_id).Update("last_last_workout", user.LastWorkout)
	when := time.Now()
	if daysago != 0 {
		when = time.Now().Add(time.Duration(-24*daysago) * time.Hour)
	}
	db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("last_workout", when)
	workout := &Workout{
		UserID: user.ID,
	}
	db.Model(&user).Association("Workouts").Append(workout)
	return nil
}
