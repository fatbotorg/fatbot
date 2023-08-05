package users

import (
	"fatbot/db"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"

	"gorm.io/gorm"
)

type NoWorkoutsError struct {
	Name string
}

func (e *NoWorkoutsError) Error() string {
	return fmt.Sprintf("%s has no workouts", e.Name)
}

type Workout struct {
	gorm.Model
	UserID         uint
	GroupID        uint
	PhotoMessageID int
	Flagged        bool
	Streak         int
}

func getLastCycleExactTime() time.Time {
	reportHour := viper.GetInt("report.hour")
	reportWeekDay := viper.GetString("report.day")
	timezone := viper.GetString("timezone")
	location, _ := time.LoadLocation(timezone)
	var lastCycleStartDate time.Time
	if time.Now().Weekday().String() == reportWeekDay && time.Now().In(location).Hour() >= reportHour {
		lastCycleStartDate = time.Now()
	} else {
		daysSinceCycleStart := int(time.Now().Weekday()) + 1
		lastCycleStartDate = time.Now().AddDate(0, 0, -int(daysSinceCycleStart))
	}
	return time.Date(
		lastCycleStartDate.Year(),
		lastCycleStartDate.Month(),
		lastCycleStartDate.Day(),
		reportHour, 0, 0, 0,
		location)
}

func (user *User) LoadWorkoutsThisCycle(chatId int64) error {
	lastCycleExactTime := getLastCycleExactTime()
	group, err := GetGroup(chatId)
	if err != nil {
		return err
	}
	db := db.DBCon
	if err := db.Model(&User{}).
		Preload(
			"Workouts",
			"created_at > ? AND group_id = ? AND flagged = ?",
			lastCycleExactTime,
			group.ID,
			false,
		).Find(&user, "telegram_user_id = ?", user.TelegramUserID).Error; err != nil {
	}
	return nil
}

func (user *User) GetPastWeekWorkouts(chatId int64) []Workout {
	db := db.DBCon
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

func (user *User) GetPreviousWeekWorkouts(chatId int64) []Workout {
	db := db.DBCon
	previousWeeksStart := time.Now().Add(time.Duration(-14) * time.Hour * 24)
	previousWeeksEnd := time.Now().Add(time.Duration(-7) * time.Hour * 24)
	group, err := GetGroup(chatId)
	if err != nil {
		log.Error(err)
		sentry.CaptureException(err)
		return []Workout{}
	}
	if err := db.Model(&User{}).
		Preload("Workouts", "created_at > ? AND created_at < ? AND group_id = ? AND flagged = ?",
			previousWeeksStart, previousWeeksEnd, group.ID, false).
		Find(&user, "telegram_user_id = ?", user.TelegramUserID).Error; err != nil {
	}
	log.Debug(user.Workouts)
	return user.Workouts
}
func (user *User) GetMonthlyWorkoutsSummary(chatId int64, start, end time.Time) (userWorkoutsByMonthAsString []string) {
	db := db.DBCon
	group, err := GetGroup(chatId)
	if err != nil {
		log.Error(err)
		sentry.CaptureException(err)
		return nil
	}
	if err := db.Model(&User{}).
		Preload("Workouts", "created_at > ? AND created_at < ? AND group_id = ? AND flagged = ?",
			start, end, group.ID, false).
		Find(&user, "telegram_user_id = ?", user.TelegramUserID).Error; err != nil {
		log.Error(err)
		sentry.CaptureException(err)
		return nil
	}
	workoutsCountByMonth := make(map[time.Month]int)

	// Iterate over the user's workouts and add them to the corresponding month's slice
	for _, workout := range user.Workouts {
		month := workout.CreatedAt.Month()
		workoutsCountByMonth[month]++
	}
	// Create a slice to store the workout counts as strings
	userWorkoutsByMonthAsString = make([]string, 0, len(workoutsCountByMonth))
	for i, count := range workoutsCountByMonth {
		userWorkoutsByMonthAsString[i] = fmt.Sprintf("%d", count)
	}
	log.Debug(user.Workouts)
	return
}

func (user *User) FlagLastWorkout(chatId int64) error {
	db := db.DBCon
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
	db := db.DBCon
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
	db := db.DBCon
	workout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		return err
	}
	db.Model(&workout).
		Update("created_at", workout.CreatedAt.Add(time.Duration(-days*24*int64(time.Hour))))
	return nil
}

func isTodayOrWasYesterday(someDate time.Time) bool {
	today := time.Now().YearDay()
	workout := someDate.YearDay()
	if today == 1 {
		return workout == 1 || workout == 365
	}
	return today-1 == workout || today == workout
}

func (user *User) UpdateWorkout(update tgbotapi.Update, lastWorkout Workout) (Workout, error) {
	db := db.DBCon
	messageId := update.Message.MessageID
	db.Where("telegram_user_id = ?", user.TelegramUserID).Find(&user)
	group, err := GetGroup(update.FromChat().ID)
	if err != nil {
		return lastWorkout, err
	}

	var streak int
	if isTodayOrWasYesterday(lastWorkout.CreatedAt) {
		if lastWorkout.Streak > 0 {
			streak = lastWorkout.Streak + 1
		} else {
			streak = 2
		}
	}

	workout := &Workout{
		UserID:         user.ID,
		PhotoMessageID: messageId,
		GroupID:        group.ID,
		Streak:         streak,
	}
	db.Model(&user).Association("Workouts").Append(workout)
	return *workout, nil
}

func (workout *Workout) IsOlderThan(minutes int) bool {
	diffInMinutes := int(time.Now().Sub(workout.CreatedAt).Minutes())
	return diffInMinutes > minutes
}

func (user *User) GetLastXWorkout(lastx int, chatId int64) (Workout, error) {
	db := db.DBCon
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
		return Workout{}, &NoWorkoutsError{Name: user.GetName()}
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
