package updates

import (
	"fatbot/schedule"
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

func handleCommandUpdate(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	if !update.FromChat().IsPrivate() {
		return nil
	}
	if isAdminCommand(update.Message.Command()) {
		return handleAdminCommandUpdate(fatBotUpdate)
	}
	var err error
	var msg tgbotapi.MessageConfig
	msg.Text = "Unknown command"
	switch update.Message.Command() {
	case "join", "start":
		msg, err = handleJoinCommand(fatBotUpdate)
		if err != nil {
			return err
		}
	case "status":
		msg = handleStatusCommand(update)
	case "help":
		msg.ChatID = update.FromChat().ID
		msg.Text = "Join the group using: /join\nCheck your status using: /status"
	default:
		msg.ChatID = update.FromChat().ID
	}
	if msg.Text == "" {
		return nil
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func handleNewGroupCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	groupChatId := update.Message.Chat.ID
	groupChatTitle := update.Message.Chat.Title
	msg := tgbotapi.NewMessage(groupChatId, "")
	switch update.FromChat().Type {
	case "private":
		return msg
	case "group":
		msg.Text = "not a supergroup, try creating an anonymous admin"
	case "supergroup":
		if err := users.CreateGroup(groupChatId, groupChatTitle); err != nil {
			log.Error("could not create group %s, id: %d", groupChatTitle, groupChatId)
			msg.Text = ""
		}
		msg.Text = fmt.Sprintf("group %s ready to start🏋️", groupChatTitle)
	}
	return msg
}

func handleStatusCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	var user users.User
	var err error
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	if user, err = users.GetUserFromMessage(update.Message); err != nil {
		log.Error(err)
		sentry.CaptureException(err)
	} else if user.ID == 0 {
		msg.Text = "Unregistered user"
		return msg
	}
	chatId, err := user.GetChatId()
	// FIX: user typed errors not this
	if err != nil {
		if err.Error() == "user has multiple groups - ambiguate" {
			if chatIds, err := user.GetChatIds(); err != nil {
				log.Error(err)
				sentry.CaptureException(err)
				return msg
			} else {
				for _, chatId := range chatIds {
					group, _ := users.GetGroup(chatId)
					msg.Text = msg.Text +
						"\n\n" +
						fmt.Sprint(group.Title) +
						": " +
						createStatusMessage(user, chatId, msg).Text
				}
			}
			return msg
		} else {
			log.Error(err)
			sentry.CaptureException(err)
			return msg
		}
	}
	msg = createStatusMessage(user, chatId, msg)
	return msg
}

func createStatusMessage(user users.User, chatId int64, msg tgbotapi.MessageConfig) tgbotapi.MessageConfig {
	lastWorkout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		log.Errorf("Err getting last workout: %s", err)
		sentry.CaptureException(err)
		return msg
	}
	if lastWorkout.CreatedAt.IsZero() {
		log.Warn("no last workout")
		msg.Text = "I don't have your last workout yet."
	} else {
		currentTime := time.Now()
		diff := currentTime.Sub(lastWorkout.CreatedAt)
		days := int(5 - diff.Hours()/24)
		msg.Text = fmt.Sprintf("%s, your last workout was on %s\nYou have %d days and %d hours left.",
			user.GetName(),
			lastWorkout.CreatedAt.Weekday(),
			days,
			120-int(diff.Hours())-24*days-1,
		)
	}
	return msg
}

func hasValidGroupCommandArgument(commandArguments string) (bool, users.Group) {
	joinArguments := strings.Split(commandArguments, " ")
	groupTitle := joinArguments[0]
	if group, err := users.GetGroupByTitle(groupTitle); err != nil {
		return false, users.Group{}
	} else {
		return true, group
	}
}

func getNameFromUpdate(update tgbotapi.Update) string {
	userName := update.SentFrom().UserName
	if userName != "" {
		return userName
	} else {
		return fmt.Sprintf("%s %s", update.SentFrom().FirstName, update.SentFrom().LastName)
	}
}

func sendLinkJoinForAdminApproval(fatBotUpdate FatBotUpdate, group users.Group) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	userId := fatBotUpdate.Update.FromChat().ID
	name := getNameFromUpdate(fatBotUpdate.Update)
	adminMessage := tgbotapi.NewMessage(0, fmt.Sprintf("%s wants to join using a link to %s, please approve", name, group.Title))
	var approvalKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprintf("%d %d %s %s", group.ChatID, userId, name, fatBotUpdate.Update.SentFrom().UserName)),
			tgbotapi.NewInlineKeyboardButtonData("Block", fmt.Sprintf("%s %d", "block", userId)),
		),
	)
	adminMessage.ReplyMarkup = approvalKeyboard
	users.SendMessageToAdmins(fatBotUpdate.Bot, adminMessage)
	text := `Hello and welcome!
You will soon get a link to join the group 🎉.
Once you click the link, please send a picture of your workout *in the group chat*
‼️ NOTE ‼️: if you don’t send a picture in the same day you get the link you will be BANNED from the group!`
	msg.Text = text
	return msg, nil
}

func handleJoinCommand(fatBotUpdate FatBotUpdate) (msg tgbotapi.MessageConfig, err error) {
	if user, err := users.GetUserById(fatBotUpdate.Update.SentFrom().ID); err != nil {
		if _, classificationErr := err.(*users.NoSuchUserError); classificationErr {
			return handleJoinCommandNewUser(fatBotUpdate)
		}
		return msg, err
	} else {
		return handleJoinCommandExistingUser(fatBotUpdate, user)
	}
}

func handleJoinCommandNewUser(fatBotUpdate FatBotUpdate) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	args := fatBotUpdate.Update.Message.CommandArguments()
	if hasValidGroup, group := hasValidGroupCommandArgument(args); hasValidGroup {
		return sendLinkJoinForAdminApproval(fatBotUpdate, group)
	}
	from := fatBotUpdate.Update.Message.From
	adminMessage := tgbotapi.NewMessage(0,
		fmt.Sprintf(
			"New join request: %s %s %s, choose a group",
			from.FirstName, from.LastName, from.UserName,
		),
	)
	adminMessage.ReplyMarkup = createNewUserGroupsKeyboard(from.ID, from.FirstName, from.UserName)
	users.SendMessageToAdmins(fatBotUpdate.Bot, adminMessage)
	text := `Hello and welcome!
You will soon get a link to join the group 🎉.
Once you click the link, please send a picture of your workout *in the group chat*
‼️ NOTE ‼️: if you don’t send a picture in the same day you get the link you will be BANNED from the group!`
	msg.Text = text
	return msg, nil
}

func handleJoinCommandExistingUser(fatBotUpdate FatBotUpdate, user users.User) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	if user.Active {
		msg.Text = "You are already active"
		return msg, nil
	}
	lastBanDate, err := user.GetLastBanDate()
	if err != nil {
		return msg, err
	}
	timeSinceBan := int(time.Now().Sub(lastBanDate).Hours())
	waitHours := viper.GetInt("ban.wait.hours")
	if timeSinceBan < waitHours {
		msg.Text = fmt.Sprintf("%s, it's only been %d hours, you have to wait %d", user.GetName(), timeSinceBan, waitHours)
	} else {
		if err := user.Rejoin(fatBotUpdate.Update, fatBotUpdate.Bot); err != nil {
			return msg, err
		}
	}
	return
}

func handleAdminCommandUpdate(fatBotUpdate FatBotUpdate) error {
	var msg tgbotapi.MessageConfig
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	msg.ChatID = update.FromChat().ID

	user, err := users.GetUserById(update.Message.From.ID)
	if err != nil {
		return err
	}
	if !user.IsAdmin {
		return nil
	}
	switch update.Message.Command() {
	case "admin":
		msg = state.HandleAdminCommand(update)
	case "admin_send_report":
		schedule.CreateChart(bot)
	default:
		msg.Text = "Unknown command"
	}
	if msg.Text == "" {
		return nil
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}
