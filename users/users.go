package users

import (
	"fmt"
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
	Active            bool
	IsAdmin           bool
	Workouts          []Workout
	NotificationCount int
	BanCount          int
	LeaderCount       int
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
	db.Where("deactivated = ?", 0).Find(&users)
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

func GetUserFromMessage(message *tgbotapi.Message) (User, error) {
	db := getDB()
	var user User
	db.Where(User{
		Username:       message.From.UserName,
		Name:           message.From.FirstName,
		TelegramUserID: message.From.ID,
	}).FirstOrCreate(&user)
	if user.ChatID == user.TelegramUserID || user.ChatID == 0 {
		if err := db.Model(&user).
			Where("telegram_user_id = ?", user.TelegramUserID).
			Update("chat_id", message.Chat.ID).Error; err != nil {
			return user, err
		}
	}
	return user, nil
}

func (user *User) UpdateInactive() error {
	db := getDB()
	if err := db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Updates(User{
		Active: false,
	}).Error; err != nil {
		return err
	}
	return nil
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

// TODO:
// Think about how to createa group-based leader so that it in the context of one group
//
//	func (user *User) IncrementLeaderCount() error {
//		db := getDB()
//		if err := db.Model(&user).
//			Where("telegram_user_id = ?", user.TelegramUserID).
//			Updates(User{
//				LeaderCount: user.LeaderCount + 1,
//			}).Error; err != nil {
//			return err
//		}
//		return nil
//	}

func (user *User) Ban(bot *tgbotapi.BotAPI) error {
	banChatMemberConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID:             user.ChatID,
			SuperGroupUsername: "shotershmenimbot",
			ChannelUsername:    "",
			UserID:             user.TelegramUserID,
		},
		UntilDate:      0,
		RevokeMessages: false,
	}
	_, err := bot.Request(banChatMemberConfig)
	if err != nil {
		return err
	}
	if err := user.UpdateInactive(); err != nil {
		return fmt.Errorf("Error with updating inactivity: %s ban count: %s",
			user.GetName(), err)
	}
	if err != user.IncrementBanCount() {
		return fmt.Errorf("Error with bumping user %s ban count: %s",
			user.GetName(), err)
	}
	msg := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf("It's been 5 full days since %s worked out.\nI kicked them", user.GetName()))
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (user *User) UnBan(bot *tgbotapi.BotAPI) error {
	unbanConfig := tgbotapi.UnbanChatMemberConfig{
		ChatMemberConfig: user.CreateChatMemberConfig(bot.Self.UserName),
	}
	if _, err := bot.Request(unbanConfig); err != nil {
		return err
	}
	return nil
}

func (user *User) Invite(bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(user.TelegramUserID, "")
	unixTime24HoursFromNow := int(time.Now().Add(time.Duration(24 * time.Hour)).Unix())
	chatConfig := tgbotapi.ChatConfig{
		ChatID:             user.ChatID,
		SuperGroupUsername: bot.Self.UserName,
	}
	createInviteLinkConfig := tgbotapi.CreateChatInviteLinkConfig{
		ChatConfig:         chatConfig,
		Name:               user.GetName(),
		ExpireDate:         unixTime24HoursFromNow,
		MemberLimit:        1,
		CreatesJoinRequest: false,
	}
	response, err := bot.Request(createInviteLinkConfig)
	if err != nil {
		return err
	}
	var dat map[string]interface{}
	json.Unmarshal(response.Result, &dat)
	for k, v := range dat {
		if k == "invite_link" {
			msg.Text = msg.Text + fmt.Sprint(v)
		}
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}
