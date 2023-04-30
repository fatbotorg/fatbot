package users

import (
	"time"
)

func (user *User) GetWorkouts() []Workout {
	db := getDB()
	if err := db.Model(&User{}).Where("telegram_user_id = ?", user.TelegramUserID).
		Preload("Workouts").Find(&user).Error; err != nil {
	}
	return user.Workouts
}

// TODO:
// Remove last_last field and use workouts table
func (user *User) RollbackLastWorkout() (Workout, error) {
	db := getDB()
	if err := db.Where(User{TelegramUserID: user.TelegramUserID}).First(&user).Error; err != nil {
		return Workout{}, err
	} else {
		lastWorkout, err := user.GetLastWorkout()
		if err != nil {
			return Workout{}, err
		}
		db.Delete(&Workout{}, lastWorkout.ID)
	}
	newLastWorkout, err := user.GetLastWorkout()
	if err != nil {
		return Workout{}, err
	}
	return newLastWorkout, nil
}

func (user *User) UpdateWorkout(messageId int) error {
	db := getDB()
	db.Where("telegram_user_id = ?", user.TelegramUserID).Find(&user)
	when := time.Now()
	// TODO:
	// Allow old updates
	//
	// if daysago != 0 {
	// 	when = time.Now().Add(time.Duration(-24*daysago) * time.Hour)
	// }
	db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("last_workout", when)
	db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("was_notified", 0)
	workout := &Workout{
		UserID:         user.ID,
		PhotoMessageID: messageId,
	}
	db.Model(&user).Association("Workouts").Append(workout)
	return nil
}
