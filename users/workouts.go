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
	WhoopID        string
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

func (user *User) LoadWorkoutsThisMonthlyCycle(chatId int64) error {
	timezone := viper.GetString("timezone")
	location, _ := time.LoadLocation(timezone)
	thisMonthsFirstDay := time.Date(
		time.Now().Year(),
		time.Now().Month(),
		1,
		0, 0, 0, 0,
		location)
	group, err := GetGroup(chatId)
	if err != nil {
		return err
	}
	db := db.DBCon
	if err := db.Model(&User{}).
		Preload(
			"Workouts",
			"created_at > ? AND group_id = ? AND flagged = ?",
			thisMonthsFirstDay,
			group.ID,
			false,
		).Find(&user, "telegram_user_id = ?", user.TelegramUserID).Error; err != nil {
	}
	log.Debug("loaded", "groupid", group.ID, "user", user.Name, "workouts", len(user.Workouts))
	log.Debug("this month firstday", "firstday", thisMonthsFirstDay)
	return nil
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

func (user *User) LoadWorkoutsReportCycle(chatId int64) error {
	// The report runs at the start of a new cycle, so we want the PREVIOUS cycle.
	// getLastCycleExactTime() returns the start of the CURRENT cycle (e.g., Sat 20:00 Today).
	cycleFinish := getLastCycleExactTime()
	cycleStart := cycleFinish.AddDate(0, 0, -7)

	group, err := GetGroup(chatId)
	if err != nil {
		return err
	}
	db := db.DBCon
	if err := db.Model(&User{}).
		Preload(
			"Workouts",
			"created_at > ? AND created_at <= ? AND group_id = ? AND flagged = ?",
			cycleStart,
			cycleFinish,
			group.ID,
			false,
		).Find(&user, "telegram_user_id = ?", user.TelegramUserID).Error; err != nil {
		return err
	}
	return nil
}

func (user *User) GetPreviousWeekWorkouts(chatId int64) []Workout {
	db := db.DBCon
	timezone := viper.GetString("timezone")
	location, _ := time.LoadLocation(timezone)
	previousWeeksStart := time.Now().In(location).Add(time.Duration(-14) * time.Hour * 24)
	previousWeeksEnd := time.Now().In(location).Add(time.Duration(-7) * time.Hour * 24)
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

func (user *User) RollbackLastWorkout(chatId int64) (Workout, Workout, error) {
	db := db.DBCon
	lastWorkout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		return Workout{}, Workout{}, err
	}
	db.Delete(&Workout{}, lastWorkout.ID)
	newLastWorkout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		return lastWorkout, Workout{}, err
	}
	return lastWorkout, newLastWorkout, nil
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

func IsTodayOrWasYesterday(someDate time.Time) bool {
	today := time.Now().YearDay()
	workout := someDate.YearDay()
	if today == 1 {
		return workout == 1 || workout == 365
	}
	return today-1 == workout || today == workout
}

func IsSameDay(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func (user *User) CreateDummyWorkout() {
	db := db.DBCon
	db.Where("telegram_user_id = ?", user.TelegramUserID).Find(&user)
	user.LoadGroups()
	for _, group := range user.Groups {
		workout := &Workout{
			UserID:  user.ID,
			Flagged: true,
			GroupID: group.ID,
		}
		if err := db.Model(&user).Association("Workouts").Append(workout); err != nil {
			log.Error("failed to create dummy workout", err, "user", user.GetName())
		}
	}
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
	if IsTodayOrWasYesterday(lastWorkout.CreatedAt) {
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

func (user *User) GetLastWorkout() (Workout, error) {
	db := db.DBCon
	err := db.Model(&User{}).Preload("Workouts", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC").Limit(1)
	}).Find(&user).Error
	if err != nil {
		return Workout{}, err
	}
	if len(user.Workouts) > 0 {
		return user.Workouts[0], nil
	}
	return Workout{}, &NoWorkoutsError{Name: user.GetName()}
}

func (user *User) GetLastXWorkout(lastx int, chatId int64) (Workout, error) {
	db := db.DBCon
	group, err := GetGroup(chatId)
	if err != nil {
		return Workout{}, err
	}
	if err := db.Model(&User{}).
		Preload("Workouts", func(db *gorm.DB) *gorm.DB {
			return db.Where("group_id = ?", group.ID).Order("created_at ASC")
		}).
		Limit(lastx).
		Find(&user).Error; err != nil {
		return Workout{}, err
	}
	if len(user.Workouts) == 0 || lastx > len(user.Workouts) {
		return Workout{}, &NoWorkoutsError{Name: user.GetName()}
	}
	return user.Workouts[len(user.Workouts)-lastx], nil
}

func (user *User) GetWorkoutInTimeRange(start, end time.Time) (Workout, error) {
	db := db.DBCon
	var workout Workout
	err := db.Where("user_id = ? AND created_at BETWEEN ? AND ?", user.ID, start, end).First(&workout).Error
	return workout, err
}
