package state

import (
	"fatbot/users"
	"fmt"
	"strconv"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ActionData struct {
	Data   string
	Update tgbotapi.Update
	Bot    *tgbotapi.BotAPI
	State  *State
}

func (menu RenameMenu) PerformAction(params ActionData) error {
	state := params.State
	telegramUserId, err := state.getTelegramUserId()
	if err != nil {
		return err
	}
	if user, err := users.GetUserById(telegramUserId); err != nil {
		return err
	} else {
		user.Rename(params.Data)
	}
	return nil
}

func (menu PushWorkoutMenu) PerformAction(params ActionData) error {
	telegramUserId, err := params.State.getTelegramUserId()
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}
	if user, err := users.GetUserById(telegramUserId); err != nil {
		return err
	} else {
		days, err := strconv.ParseInt(params.Data, 10, 64)
		if err != nil {
			return err
		}
		if err := user.PushWorkout(days, groupChatId); err != nil {
			return err
		}
	}
	return nil
}

func (menu DeleteLastWorkoutMenu) PerformAction(params ActionData) error {
	groupChatId, err := params.State.getGroupChatId()
	userId, _ := strconv.ParseInt(params.Data, 10, 64)
	user, err := users.GetUserById(userId)
	if err != nil {
		return err
	}
	if newLastWorkout, err := user.RollbackLastWorkout(groupChatId); err != nil {
		return err
	} else {
		msg := tgbotapi.NewMessage(params.Update.FromChat().ID, "")
		message := fmt.Sprintf("Deleted last workout for user %s\nRolledback to: %s",
			user.GetName(), newLastWorkout.CreatedAt.Format("2006-01-02 15:04:05"))
		msg.Text = message
		if _, err := params.Bot.Send(msg); err != nil {
			return err
		}
		messageToUser := tgbotapi.NewMessage(0,
			fmt.Sprintf("Your last workout was cancelled by the admin.\nUpdated workout: %s",
				newLastWorkout.CreatedAt))
		user.SendPrivateMessage(params.Bot, messageToUser)
	}
	return nil
}

func (menu ShowUsersMenu) PerformAction(params ActionData) error {
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}
	users := users.GetGroupWithUsers(groupChatId).Users
	msg := tgbotapi.NewMessage(params.Update.FromChat().ID, "")
	message := ""
	var lastWorkoutStr string
	for _, user := range users {
		if !user.Active {
			continue
		}
		lastWorkout, err := user.GetLastXWorkout(1, groupChatId)
		if err != nil {
			log.Errorf("Err getting last workout: %s", err)
			continue
		}
		if lastWorkout.CreatedAt.IsZero() {
			lastWorkoutStr = "no record"
		} else {
			hour, min, _ := lastWorkout.CreatedAt.Clock()
			lastWorkoutStr = fmt.Sprintf("%s, %d:%d", lastWorkout.CreatedAt.Weekday().String(), hour, min)
		}
		msg.Text = message + fmt.Sprintf("%s [%s]", user.GetName(), lastWorkoutStr) + "\n"
	}
	if msg.Text == "" {
		msg.Text = "No users to show"
	}
	if _, err := params.Bot.Send(msg); err != nil {
		return err
	}
	return nil
}
