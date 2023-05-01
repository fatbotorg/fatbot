package users

import (
	"os"

	"github.com/charmbracelet/log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username          string
	Name              string
	NickName          string
	ChatID            int64
	TelegramUserID    int64
	IsActive          bool
	IsAdmin           bool
	Workouts          []Workout
	NotificationCount int
	BanCount          int
}

type Workout struct {
	gorm.Model
	Cancelled      bool
	UserID         uint
	PhotoMessageID int
}

func getDB() *gorm.DB {
	path := os.Getenv("DBPATH")
	if path == "" {
		path = "fat.db"
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	return db
}

// BUG:
// returns the entire DB, needs filtering by chat_id
func GetUsers() []User {
	db := getDB()
	var users []User
	db.Find(&users)
	return users
}

func (user *User) SendPrivateMessage(bot *tgbotapi.BotAPI, text string) error {
	msg := tgbotapi.NewMessage(user.TelegramUserID, text)
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (user *User) GetName() (name string) {
	if user.NickName != "" {
		name = user.NickName
	} else {
		name = user.Name
	}
	return
}

func GetUserById(userId int64) (user User, err error) {
	db := getDB()
	if err := db.Model(&user).Where("telegram_user_id = ?", userId).Find(&user).Error; err != nil {
		return user, err
	}
	return user, nil
}

func GetUserFromMessage(message *tgbotapi.Message) User {
	db := getDB()
	var user User
	db.Where(User{
		Username:       message.From.UserName,
		Name:           message.From.FirstName,
		TelegramUserID: message.From.ID,
	}).FirstOrCreate(&user)
	if user.ChatID == user.TelegramUserID || user.ChatID == 0 {
		db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("chat_id", message.Chat.ID)
	}
	return user
}

func (user *User) UpdateInactive() {
	db := getDB()
	db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("is_active", false)
	db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("was_notified", false)
}

func (user *User) Rename(name string) error {
	db := getDB()
	if err := db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Update("nick_name", name).Error; err != nil {
		return err
	}
	return nil
}

func (user *User) IncrementNotificationCount() error {
	db := getDB()
	if err := db.Model(&user).
		Where("telegram_user_id = ?", user.TelegramUserID).
		Updates(User{
			NotificationCount: user.NotificationCount + 1,
		}).Error; err != nil {
		return err
	}
	return nil
}

func (user *User) IncrementBanCount() error {
	db := getDB()
	if err := db.Model(&user).
		Where("telegram_user_id = ?", user.TelegramUserID).
		Updates(User{
			BanCount: user.BanCount + 1,
		}).Error; err != nil {
		return err
	}
	return nil
}
