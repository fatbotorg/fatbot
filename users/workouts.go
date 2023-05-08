package users

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Workout struct {
	gorm.Model
	UserID         uint
	PhotoMessageID int
	GroupID        uint
}

func (user *User) GetPastWeekWorkouts(chatId int64) []Workout {
	db := getDB()
	lastWeek := time.Now().Add(time.Duration(-7) * time.Hour * 24)
	if err := db.Model(&User{}).
		Preload("Groups", "chat_id = ?", chatId).
		Preload("Workouts", "created_at > ?", lastWeek).
		Find(&user, "telegram_user_id = ? AND group.chat_id = ?", user.TelegramUserID, chatId).Error; err != nil {
	}
	return user.Workouts
}

func (user *User) RollbackLastWorkout(chatId int64) (Workout, error) {
	db := getDB()
	lastWorkout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		return Workout{}, err
	}
	db.Delete(&Workout{}, lastWorkout.ID)
	newLastWorkout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		return Workout{}, err
	}
	return newLastWorkout, nil
}

func (user *User) PushWorkout(days, chatId int64) error {
	db := getDB()
	workout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		return err
	}
	db.Model(&workout).
		Update("created_at", workout.CreatedAt.Add(time.Duration(-days*24*int64(time.Hour))))
	return nil
}

func (user *User) UpdateWorkout(messageId int) error {
	db := getDB()
	db.Where("telegram_user_id = ?", user.TelegramUserID).Find(&user)
	db.Model(&user).Update("was_notified", 0)
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

func (user *User) GetLastXWorkout(lastx int, chatId int64) (Workout, error) {
	db := getDB()
	if err := db.Model(&User{}).
		Preload("Groups", "chat_id = ?", chatId).
		Preload("Workouts").Limit(lastx).Find(&user).Error; err != nil {
		return Workout{}, err
	}
	if len(user.Workouts) == 0 || lastx > len(user.Workouts) {
		return Workout{}, fmt.Errorf("No workouts, or not enough: %s", user.GetName())
	}
	return user.Workouts[len(user.Workouts)-lastx], nil
}
