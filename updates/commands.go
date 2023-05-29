package updates

import (
	"fatbot/schedule"
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	case "join":
		msg, err = handleJoinCommand(fatBotUpdate)
		if err != nil {
			return err
		}
	case "status":
		msg = handleStatusCommand(update)
	case "help":
		msg.ChatID = update.FromChat().ID
		msg.Text = "/status\n/join"
	default:
		msg.ChatID = update.FromChat().ID
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
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

func handleJoinCommand(fatBotUpdate FatBotUpdate) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	if user, err := users.GetUserById(fatBotUpdate.Update.SentFrom().ID); err != nil {
		return msg, err
	} else if user.ID == 0 {
		from := fatBotUpdate.Update.Message.From
		adminMessage := tgbotapi.NewMessage(0,
			fmt.Sprintf(
				"User: %s %s %s is new and wants to join a group, where to?",
				from.FirstName, from.LastName, from.UserName,
			),
		)
		adminMessage.ReplyMarkup = createNewUserGroupsKeyboard(from.ID, from.FirstName, from.UserName)
		users.SendMessageToAdmins(fatBotUpdate.Bot, adminMessage)
		msg.Text = "Welcome! I've sent your request to the admins"
		return msg, nil
	} else {
		if user.Active {
			msg.Text = "You are already active"
			return msg, nil
		}
		lastBanDate, err := user.GetLastBanDate()
		if err != nil {
			return msg, err
		}
		timeSinceBan := int(time.Now().Sub(lastBanDate).Hours())
		if timeSinceBan < 48 {
			msg.Text = fmt.Sprintf("%s, it's only been %d hours, you have to wait 48", user.GetName(), timeSinceBan)
		} else {
			msg.Text = fmt.Sprintf("Hi %s, welcome back I'm sending this for admin approval", user.GetName())
			adminMessage := tgbotapi.NewMessage(0, fmt.Sprintf("User %s wants to rejoin his group do you approve?", user.GetName()))
			var approvalKeyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprint(user.ID)),
					tgbotapi.NewInlineKeyboardButtonData("Decline", "false"),
				),
			)
			adminMessage.ReplyMarkup = approvalKeyboard
			users.SendMessageToAdmins(fatBotUpdate.Bot, adminMessage)
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
