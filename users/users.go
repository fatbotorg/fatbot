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
	OnProbation    bool
	Workouts       []Workout
	Events         []Event
	Groups         []*Group `gorm:"many2many:user_groups;"`
}

type Blacklist struct {
	gorm.Model
	TelegramUserID int64
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

func (user *User) LoadGroups() (*User, error) {
	err := getDB().Preload("Groups").Find(&user).Error
	if err != nil {
		return &User{}, err
	} else {
		return user, nil
	}
}

func GetUser(id uint) (user User, err error) {
	db := getDB()
	if err := db.Find(&user, id).Error; err != nil {
		return user, err
	}
	if len(user.Groups) == 0 {
		group, err := GetGroup(user.ChatID)
		if err != nil {
			return User{}, err
		}
		// NOTE: temp iteration to fill up the groups struct from the "old" chatId
		if err := db.Model(&user).Association("Groups").Append(group); err != nil {
			return user, err
		}
	}
	return user, nil
}

// BUG: #7
// returns the entire DB, needs filtering by chat_id
func GetUsers(chatId int64) []User {
	db := getDB()
	var users []User
	if chatId == 0 {
		db.Where("active = ?", true).Find(&users)
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
	if err := db.Model(&user).
		Preload("Groups").
		Where("telegram_user_id = ?", userId).
		Find(&user).Error; err != nil {
		return user, err
	}
	return user, nil
}

func (user *User) create() error {
	db := getDB()
	return db.Create(&user).Error
}

func GetUserFromMessage(message *tgbotapi.Message) (User, error) {
	db := getDB()
	var user User
	if err := db.Where(User{
		Username:       message.From.UserName,
		Name:           message.From.FirstName,
		TelegramUserID: message.From.ID,
	}).Find(&user).Error; err != nil {
		return user, err
	}
	return user, nil
}

func (user *User) UpdateActive(should bool) error {
	db := getDB()
	if err := db.Model(&user).Update("active", should).Error; err != nil {
		return err
	}
	return nil
}

func (user *User) UpdateOnProbation(probation bool) error {
	db := getDB()
	if err := db.Model(&user).
		Update("on_probation", probation).Error; err != nil {
		return err
	}
	return nil
}

func (user *User) Rename(name string) error {
	db := getDB()
	if err := db.Model(&user).
		Update("nick_name", name).Error; err != nil {
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

func (user *User) Ban(bot *tgbotapi.BotAPI) (errors []error) {
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
		errors = append(errors, err)
	} else if chatMemeber.WasKicked() {
		log.Debug("Func Ban", "wasKicked", chatMemeber.WasKicked())
		return nil
	}
	_, err := bot.Request(banChatMemberConfig)
	if err != nil {
		errors = append(errors, err)
	}
	if err := user.UpdateActive(false); err != nil {
		log.Debug("Func Ban", "updateActive", false)
		errors = append(errors, fmt.Errorf("Error with updating inactivity: %s ban count: %s", user.GetName(), err))

	}
	if err := user.RegisterBanEvent(); err != nil {
		log.Errorf("Error while registering ban event: %s", err)
	}
	messagesToSend := []tgbotapi.MessageConfig{}
	groupMessage := tgbotapi.NewMessage(user.ChatID, fmt.Sprintf(
		"%s was not working out. 🦥⛔",
		user.GetName(),
	))
	userMessage := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf(
		`%s You were banned from the group after not working out.
		You can rejoin using /join but only in 48 hours.
		Note that once approved, you'll have 60 minutes to send 2 workouts.
		Please don't hit the command more than once or you'll get another 24 hours delay...`,
		user.GetName(),
	))
	messagesToSend = append(messagesToSend, groupMessage)
	messagesToSend = append(messagesToSend, userMessage)
	for _, msg := range messagesToSend {
		if _, err := bot.Send(msg); err != nil {
			errors = append(errors, err)
		}
	}
	return
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

func (user *User) InviteExistingUser(bot *tgbotapi.BotAPI) error {
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

func (user *User) InviteNewUser(bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(user.TelegramUserID, "")
	unixTime24HoursFromNow := int(time.Now().Add(time.Duration(24 * time.Hour)).Unix())
	chatConfig := tgbotapi.ChatConfig{
		ChatID:             user.ChatID,
		SuperGroupUsername: bot.Self.UserName,
	}
	createInviteLinkConfig := tgbotapi.CreateChatInviteLinkConfig{
		ChatConfig:         chatConfig,
		Name:               user.Name,
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
	msg.Text = "You are invited to join, but PLEASE NOTE: Upload a workout **as soon** as you join or you will be banned!" + link
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return user.create()
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

func BlockUserId(userId int64) error {
	db := getDB()
	blackListed := Blacklist{
		TelegramUserID: userId,
	}
	if err := db.Create(&blackListed); err != nil {
		log.Error(err.Error)
	}
	return nil
}

func BlackListed(id int64) bool {
	db := getDB()
	var black Blacklist
	db.Where(Blacklist{TelegramUserID: id}).Find(&black)
	if black.TelegramUserID == 0 {
		return false
	}
	return true
}
