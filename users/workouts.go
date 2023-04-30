package users

import "time"

func (user *User) GetPastWeekWorkouts() []Workout {
	db := getDB()
	lastWeek := time.Now().Add(time.Duration(-7) * time.Hour * 24)
	if err := db.Model(&User{}).Where("telegram_user_id = ?", user.TelegramUserID).
		Preload("Workouts", "created_at > ?", lastWeek).Find(&user).Error; err != nil {
	}
	return user.Workouts
}

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
	// when := time.Now()
	// TODO:
	// Allow old updates
	//
	// if daysago != 0 {
	// 	when = time.Now().Add(time.Duration(-24*daysago) * time.Hour)
	// }
	// db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("last_workout", when)
	db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("was_notified", 0)
	workout := &Workout{
		UserID:         user.ID,
		PhotoMessageID: messageId,
	}
	db.Model(&user).Association("Workouts").Append(workout)
	return nil
}

func (workout *Workout) IsOlderThan(minutes int) bool {
	diffInMinutes := int(time.Now().Sub(workout.CreatedAt).Minutes())
	return diffInMinutes > minutes
}
