package users

import (
	"fatbot/db"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"gorm.io/gorm"
)

type Workout struct {
	gorm.Model
	UserID         uint
	GroupID        uint
	PhotoMessageID int
	Flagged        bool
}

func (user *User) GetPastWeekWorkouts(chatId int64) []Workout {
	db := db.GetDB()
	lastWeek := time.Now().Add(time.Duration(-7) * time.Hour * 24)
	group, err := GetGroup(chatId)
	if err != nil {
		log.Error(err)
		sentry.CaptureException(err)
		return []Workout{}
	}
	if err := db.Model(&User{}).
		Preload("Workouts", "created_at > ? AND group_id = ? AND flagged = ?", lastWeek, group.ID, false).
		Find(&user, "telegram_user_id = ?", user.TelegramUserID).Error; err != nil {
	}
	return user.Workouts
}

func (user *User) FlagLastWorkout(chatId int64) error {
	db := db.GetDB()
	workout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		return err
	}
	err = db.Model(&workout).Update("flagged", 1).Error
	if err != nil {
		return err
	}
	return nil
}

func (user *User) RollbackLastWorkout(chatId int64) (Workout, error) {
	db := db.GetDB()
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
	db := db.GetDB()
	workout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		return err
	}
	db.Model(&workout).
		Update("created_at", workout.CreatedAt.Add(time.Duration(-days*24*int64(time.Hour))))
	return nil
}

func (user *User) UpdateWorkout(update tgbotapi.Update) error {
	db := db.GetDB()
	messageId := update.Message.MessageID
	db.Where("telegram_user_id = ?", user.TelegramUserID).Find(&user)
	group, err := GetGroup(update.FromChat().ID)
	if err != nil {
		return err
	}
	workout := &Workout{
		UserID:         user.ID,
		PhotoMessageID: messageId,
		GroupID:        group.ID,
	}
	db.Model(&user).Association("Workouts").Append(workout)
	return nil
}

func (workout *Workout) IsOlderThan(minutes int) bool {
	diffInMinutes := int(time.Now().Sub(workout.CreatedAt).Minutes())
	return diffInMinutes > minutes
}

func (user *User) GetLastXWorkout(lastx int, chatId int64) (Workout, error) {
	db := db.GetDB()
	group, err := GetGroup(chatId)
	if err != nil {
		return Workout{}, err
	}
	if err := db.Model(&User{}).
		Preload("Workouts", "group_id = ?", group.ID).
		Limit(lastx).
		Find(&user).Error; err != nil {
		return Workout{}, err
	}
	if len(user.Workouts) == 0 || lastx > len(user.Workouts) {
		return Workout{}, fmt.Errorf("No workouts, or not enough: %s", user.GetName())
	}
	return user.Workouts[len(user.Workouts)-lastx], nil
}

func (user *User) LastTwoWorkoutsInPastHour() (bool, error) {
	chatId, err := user.GetSingleChatId()
	lastWorkout, err := user.GetLastXWorkout(2, chatId)
	if err != nil {
		return false, err
	}
	return time.Now().Sub(lastWorkout.CreatedAt).Minutes() <= 60, nil
}
