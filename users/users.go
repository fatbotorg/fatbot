package users

import (
	"encoding/json"
	"fatbot/db"
	"fmt"
	"time"

	"github.com/charmbracelet/log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username       string
	Name           string
	NickName       string
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

func InitDB() error {
	db := db.DBCon
	db.AutoMigrate(&User{}, &Group{}, &Workout{}, &Event{}, &Blacklist{})
	return nil
}

func (user *User) LoadGroups() error {
	db := db.DBCon
	return db.Preload("Groups").Find(&user).Error
}

func (user *User) LoadEvents() error {
	db := db.DBCon
	return db.Preload("Events").Find(&user).Error
}

func GetUser(id uint) (user User, err error) {
	db := db.DBCon
	if err := db.Find(&user, id).Error; err != nil {
		return user, err
	}
	return
}

func GetUsers(chatId int64) []User {
	db := db.DBCon
	var users []User
	if chatId == -1 {
		db.Find(&users)
	} else if chatId == 0 {
		db.Where("active = ?", true).Find(&users)
	} else {
		users = GetGroupWithUsers(chatId).Users
	}
	return users
}

func GetInactiveUsers(chatId int64) []User {
	db := db.DBCon
	var users []User
	if chatId == -1 {
		db.Find(&users)
	} else if chatId == 0 {
		db.Where("active = ?", false).Find(&users)
	} else {
		users = GetGroupWithInactiveUsers(chatId).Users
	}
	return users
}

func GetAdminUsers() []User {
	db := db.DBCon
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
	db := db.DBCon
	if err := db.Model(&user).
		Preload("Groups").
		Where("telegram_user_id = ?", userId).
		Find(&user).Error; err != nil {
		return user, err
	}
	return user, nil
}

func (user *User) create() error {
	db := db.DBCon
	return db.Create(&user).Error
}

func GetUserFromMessage(message *tgbotapi.Message) (User, error) {
	db := db.DBCon
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
	db := db.DBCon
	if err := db.Model(&user).Update("active", should).Error; err != nil {
		return err
	}
	return nil
}

func (user *User) UpdateOnProbation(probation bool) error {
	db := db.DBCon
	if err := db.Model(&user).
		Update("on_probation", probation).Error; err != nil {
		return err
	}
	return nil
}

func (user *User) Rename(name string) error {
	db := db.DBCon
	if err := db.Model(&user).
		Update("nick_name", name).Error; err != nil {
		return err
	}
	return nil
}

func (user *User) CreateChatMemberConfig(botName string, chatId int64) tgbotapi.ChatMemberConfig {
	return tgbotapi.ChatMemberConfig{
		ChatID:             chatId,
		SuperGroupUsername: botName,
		ChannelUsername:    "",
		UserID:             user.TelegramUserID,
	}
}

func (user *User) Ban(bot *tgbotapi.BotAPI, chatId int64) (errors []error) {
	chatMemberConfig := user.CreateChatMemberConfig(bot.Self.UserName, chatId)
	getChatMemberConfig := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID:             chatId,
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
	groupMessage := tgbotapi.NewMessage(chatId, fmt.Sprintf(
		"%s was not working out. 🦥⛔",
		user.GetName(),
	))
	userMessage := tgbotapi.NewMessage(user.TelegramUserID, fmt.Sprintf(
		`%s you were banned from the group
after not working out.
You can rejoin after 48 hours:

1. Tap this: /join
2. Wait for approval
3. Get a link to join the group

*NOTICE!!* After joining the group, you
will have 60 minutes to send 2 workouts
(pictures) in the group chat.`,
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

func (user User) GetSingleChatId() (int64, error) {
	chatIds, err := user.GetChatIds()
	if err != nil {
		return 0, err
	}
	switch len(chatIds) {
	case 0:
		return 0, fmt.Errorf("cant find chatids")
	case 1:
		return chatIds[0], nil
	default:
		return 0, fmt.Errorf("too many groups cant infer")

	}
}

func (user *User) GetChatIds() (chatIds []int64, err error) {
	user.LoadGroups()
	if user.Groups == nil {
		return []int64{}, fmt.Errorf("user %s has nil groups", user.GetName())
	}
	switch len(user.Groups) {
	case 0:
		return []int64{}, fmt.Errorf("user %s has no groups", user.GetName())
	case 1:
		chatIds = []int64{
			user.Groups[0].ChatID,
		}
		return chatIds, nil
	default:
		chatIds = []int64{}
		for _, group := range user.Groups {
			chatIds = append(chatIds, group.ChatID)
		}
		return chatIds, nil
	}
}

func (user *User) GetChatId() (chatId int64, err error) {
	user.LoadGroups()
	if user.Groups == nil {
		return 0, fmt.Errorf("user %s has nil groups", user.GetName())
	}
	switch len(user.Groups) {
	case 0:
		return 0, fmt.Errorf("user %s has no groups", user.GetName())
	case 1:
		chatId = user.Groups[0].ChatID
		return chatId, nil
	default:
		return 0, fmt.Errorf("user has multiple groups - ambiguate")
	}
}

func (user *User) UnBan(bot *tgbotapi.BotAPI) error {
	chatId, err := user.GetChatId()
	if err != nil {
		return err
	}
	unbanConfig := tgbotapi.UnbanChatMemberConfig{
		ChatMemberConfig: user.CreateChatMemberConfig(bot.Self.UserName, chatId),
	}
	if _, err := bot.Request(unbanConfig); err != nil {
		return err
	}
	return nil
}

func (user *User) InviteExistingUser(bot *tgbotapi.BotAPI) error {
	chatId, err := user.GetChatId()
	if err != nil {
		return err
	}
	msg := tgbotapi.NewMessage(user.TelegramUserID, "")
	unixTime24HoursFromNow := int(time.Now().Add(time.Duration(24 * time.Hour)).Unix())
	chatConfig := tgbotapi.ChatConfig{
		ChatID:             chatId,
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

func (user *User) InviteNewUser(bot *tgbotapi.BotAPI, chatId int64) error {
	msg := tgbotapi.NewMessage(user.TelegramUserID, "")
	unixTime24HoursFromNow := int(time.Now().Add(time.Duration(24 * time.Hour)).Unix())
	chatConfig := tgbotapi.ChatConfig{
		ChatID:             chatId,
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
	db := db.DBCon
	blackListed := Blacklist{
		TelegramUserID: userId,
	}
	if err := db.Create(&blackListed); err != nil {
		log.Error(err.Error)
	}
	return nil
}

func BlackListed(id int64) bool {
	db := db.DBCon
	var black Blacklist
	db.Where(Blacklist{TelegramUserID: id}).Find(&black)
	if black.TelegramUserID == 0 {
		return false
	}
	return true
}

func (user User) GetLastBanDate() (time.Time, error) {
	db := db.DBCon
	if err := db.Preload("Events").Find(&user).Error; err != nil {
		return time.Time{}, err
	}
	var banEvents []Event
	for _, event := range user.Events {
		if event.Event == BanEventType {
			banEvents = append(banEvents, event)
		}
	}
	return banEvents[len(banEvents)-1].CreatedAt, nil
}

func (user User) Rejoin(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	adminMsg := tgbotapi.NewMessage(0, "")
	if err := user.UnBan(bot); err != nil {
		banErr := fmt.Errorf("Issue with unbanning %s: %s", user.GetName(), err)
		if !update.FromChat().IsSuperGroup() {
			log.Error(banErr)
		} else {
			return banErr
		}
	}
	if err := user.InviteExistingUser(bot); err != nil {
		return fmt.Errorf("Issue with inviting %s: %s", user.GetName(), err)
	}
	if err := user.UpdateActive(true); err != nil {
		return fmt.Errorf("Issue updating active %s: %s", user.GetName(), err)
	}
	if err := user.UpdateOnProbation(true); err != nil {
		return fmt.Errorf("Issue updating probation %s: %s", user.GetName(), err)
	}
	msg.Text = "Ok, approved"
	if adminUser, err := GetUserById(update.SentFrom().ID); err != nil {
		return err
	} else {
		adminMsg.Text = fmt.Sprintf("Approved by %s", adminUser.GetName())
		SendMessageToAdmins(bot, adminMsg)
	}
	_, err := bot.Send(msg)
	if err != nil {
		return err
	}
	return nil
}

func (user User) IsNew(chatId int64) (bool, error) {
	var noWorkouts bool
	_, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		if _, noWorkouts = err.(*NoWorkoutsError); noWorkouts {
			noWorkouts = true
		} else {
			return false, err
		}
	}
	return noWorkouts &&
		time.Now().Sub(user.CreatedAt).Hours() <= 24, nil
}
