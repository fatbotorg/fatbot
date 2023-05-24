package main

import (
	"fatbot/admin"
	"fatbot/users"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleStatusCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	var user users.User
	var err error
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	if user, err = users.GetUserFromMessage(update.Message); err != nil {
		log.Error(err)
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

func handleWorkoutUpload(update tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	var message string
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	user, err := users.GetUserById(update.SentFrom().ID)
	if err != nil {
		return msg, err
	}
	chatId := update.FromChat().ID
	if !user.IsInGroup(chatId) {
		if err := user.RegisterInGroup(chatId); err != nil {
			log.Errorf("Error registering user %s in new group %d", user.GetName(), chatId)
		}
	}
	lastWorkout, err := user.GetLastXWorkout(1, update.FromChat().ID)
	if err != nil {
		log.Warn(err)

	}
	if !lastWorkout.IsOlderThan(30) && !user.OnProbation {
		log.Warn("Workout not older than 30 minutes: %s", user.GetName())
		return msg, nil
	}
	if err := user.UpdateWorkout(update); err != nil {
		return msg, err
	}
	if lastWorkout.CreatedAt.IsZero() {
		message = fmt.Sprintf("%s nice work!\nThis is your first workout",
			user.GetName(),
		)
	} else {
		hours := time.Now().Sub(lastWorkout.CreatedAt).Hours()
		timeAgo := ""
		if int(hours/24) == 0 {
			timeAgo = fmt.Sprintf("%d hours ago", int(hours))
		} else {
			days := int(hours / 24)
			timeAgo = fmt.Sprintf("%d days and %d hours ago", days, int(hours)-days*24)
		}
		message = fmt.Sprintf("%s %s\nYour last workout was on %s (%s)",
			user.GetName(),
			users.GetRandomWorkoutMessage(),
			lastWorkout.CreatedAt.Weekday(),
			timeAgo,
		)
	}

	msg.Text = message
	msg.ReplyToMessageID = update.Message.MessageID
	return msg, nil
}

func createNewUserGroupsKeyboard(userId int64, name, username string) tgbotapi.InlineKeyboardMarkup {
	groups := users.GetGroupsWithUsers()
	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, group := range groups {
		groupLabel := fmt.Sprintf("%s", group.Title)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			groupLabel,
			fmt.Sprintf("%d %d %s %s", group.ChatID, userId, name, username),
		))
		if len(row) == 3 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 && len(row) < 3 {
		rows = append(rows, row)
	}
	blockRow := []tgbotapi.InlineKeyboardButton{}
	blockButton := tgbotapi.NewInlineKeyboardButtonData("Block", fmt.Sprintf("%s %d", "block", userId))
	blockRow = append(blockRow, blockButton)
	rows = append(rows, blockRow)

	var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
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
		admin.SendMessageToAdmins(fatBotUpdate.Bot, adminMessage)
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
			admin.SendMessageToAdmins(fatBotUpdate.Bot, adminMessage)
		}
	}
	return
}
