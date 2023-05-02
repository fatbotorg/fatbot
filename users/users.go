package users

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username       string
	Name           string
	NickName       string
	ChatID         int64
	TelegramUserID int64
	Active         bool
	IsAdmin        bool
	Workouts       []Workout
	Events         []Event
	// NotificationCount int
	// BanCount          int
	// LeaderCoun       int
}

type eventType string

const (
	Ban                 eventType = "ban"
	LastDayNotification eventType = "lastDayNotification"
	WeeklyLeader        eventType = "weeklyLeader"
)

type Event struct {
	gorm.Model
	UserID uint
	Event  eventType
}

type Workout struct {
	gorm.Model
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

func GetUser(id uint) (user User, err error) {
	db := getDB()
	if err := db.Find(&user, id).Error; err != nil {
		return user, err
	}
	return user, nil
}

// BUG: #7
// returns the entire DB, needs filtering by chat_id
func GetUsers(chatId int64) []User {
	db := getDB()
	var users []User
	if chatId == 0 {
		db.Find(&users)
	} else {
		db.Where("chat_id = ? AND active = ?", chatId, true).Find(&users)
	}
	return users
}

func GetAdminUsers() []User {
	db := getDB()
	var users []User
	db.Where("is_admin = ?", true).Find(&users)
	return users
}

func (user *User) SendPrivateMessage(bot *tgbotapi.BotAPI, messageConfig tgbotapi.MessageConfig) error {
	messageConfig.ChatID = user.TelegramUserID
	if _, err := bot.Send(messageConfig); err != nil {
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
	if err := user.UpdateActive(true); err != nil {
		return user, err
	}
	if user.ChatID == user.TelegramUserID || user.ChatID == 0 {
		if err := db.Model(&user).
			Where("telegram_user_id = ?", user.TelegramUserID).
			Updates(User{
				ChatID: message.Chat.ID,
			}).Error; err != nil {
			return user, err
		}
	}
	return user, nil
}

func (user *User) UpdateActive(should bool) error {
	db := getDB()
	if err := db.Model(&user).Where("telegram_user_id = ?", user.TelegramUserID).Updates(User{
		Active: should,
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

func (user *User) CreateChatMemberConfig(botName string) tgbotapi.ChatMemberConfig {
	return tgbotapi.ChatMemberConfig{
		ChatID:             user.ChatID,
		SuperGroupUsername: botName,
		ChannelUsername:    "",
		UserID:             user.TelegramUserID,
	}
}

func (user *User) Ban(bot *tgbotapi.BotAPI) error {
	chatMemberConfig := user.CreateChatMemberConfig(bot.Self.UserName)
	getChatMemberConfig := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID:             user.ChatID,
			SuperGroupUsername: bot.Self.UserName,
			UserID:             user.TelegramUserID,
		},
	}
	banChatMemberConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: chatMemberConfig,
		UntilDate:        0,
		RevokeMessages:   false,
	}
	if chatMemeber, err := bot.GetChatMember(getChatMemberConfig); err != nil {
		return err
	} else if chatMemeber.WasKicked() {
		return nil
	}
	_, err := bot.Request(banChatMemberConfig)
	if err != nil {
		return err
	}
	if err := user.UpdateActive(false); err != nil {
		return fmt.Errorf("Error with updating inactivity: %s ban count: %s",
			user.GetName(), err)
	}
	// TODO:
	// Add a ban event to the user
	//
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
	link, err := extractInviteLinkFromResponse(response)
	if err != nil {
		return err
	}
	msg.Text = link
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func InviteNewUser(bot *tgbotapi.BotAPI, chatId, userId int64, name string) error {
	msg := tgbotapi.NewMessage(userId, "")
	unixTime24HoursFromNow := int(time.Now().Add(time.Duration(24 * time.Hour)).Unix())
	chatConfig := tgbotapi.ChatConfig{
		ChatID:             chatId,
		SuperGroupUsername: bot.Self.UserName,
	}
	createInviteLinkConfig := tgbotapi.CreateChatInviteLinkConfig{
		ChatConfig:         chatConfig,
		Name:               name,
		ExpireDate:         unixTime24HoursFromNow,
		MemberLimit:        1,
		CreatesJoinRequest: false,
	}
	response, err := bot.Request(createInviteLinkConfig)
	if err != nil {
		return err
	}
	link, err := extractInviteLinkFromResponse(response)
	if err != nil {
		return err
	}
	msg.Text = "You are invited to join: " + link
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func extractInviteLinkFromResponse(response *tgbotapi.APIResponse) (string, error) {
	var dat map[string]interface{}
	json.Unmarshal(response.Result, &dat)
	for k, v := range dat {
		if k == "invite_link" {
			return fmt.Sprint(v), nil
		}
	}
	return "", fmt.Errorf("Could not find invite link")
}
