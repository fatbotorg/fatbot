package main

import (
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleStatefulCallback(fatBotUpdate FatBotUpdate) (err error) {
	var messageId int
	var data string
	chatId := fatBotUpdate.Update.FromChat().ID
	msg := tgbotapi.NewMessage(0, "")
	if fatBotUpdate.Update.CallbackQuery == nil {
		data = fatBotUpdate.Update.Message.Text
		messageId = fatBotUpdate.Update.Message.MessageID
		msg.Text = data
	} else {
		data = fatBotUpdate.Update.CallbackData()
		messageId = fatBotUpdate.Update.CallbackQuery.Message.MessageID
	}
	if strings.Contains(data, "adminmenu") {
		var kind state.MenuKind
		switch data {
		case "adminmenurename":
			kind = state.RenameMenuKind
		case "adminmenupushworkout":
			kind = state.PushWorkoutMenuKind
		case "adminmenudeletelastworkout":
			kind = state.DeleteLastWorkoutMenuKind
		case "adminmenushowusers":
			kind = state.ShowUsersMenuKind
		}
		if err := initAdminMenuFlow(fatBotUpdate, kind); err != nil {
			return err
		}
		return nil
	}
	msg.ChatID = chatId
	menuState, err := state.New(chatId)
	if err != nil {
		return err
	}
	value := menuState.Value + state.Delimiter + data
	if menuState.IsLastStep() {
		menuState.Value = value
		if err := menuState.PerformAction(data, *fatBotUpdate.Bot, fatBotUpdate.Update); err != nil {
			log.Error(err)
		} else {
			msg.Text = fmt.Sprintf("%s done. -> %s", menuState.Menu.Name, data)
			fatBotUpdate.Bot.Request(msg)
			err := state.DeleteStateEntry(chatId)
			if err != nil {
				log.Error(err)
			}
		}
		return nil
	}
	step := menuState.CurrentStep()
	switch step.Kind {
	case state.KeyboardStepKind:
		if len(step.Keyboard.InlineKeyboard) == 0 {
			data, _ := menuState.ExtractData()
			step.PopulateKeyboard(data)
		}
		edit := tgbotapi.NewEditMessageTextAndMarkup(
			chatId, messageId, step.Message, step.Keyboard,
		)
		if _, err := fatBotUpdate.Bot.Send(edit); err != nil {
			log.Error(err)
		}
	case state.InputStepKind:
		msg.Text = step.Message
	}
	fatBotUpdate.Bot.Request(msg)
	state.CreateStateEntry(chatId, value)
	return nil
}

func initAdminMenuFlow(fatBotUpdate FatBotUpdate, menuKind state.MenuKind) error {
	if _, err := users.GetUserById(fatBotUpdate.Update.FromChat().ID); err != nil {
		return err
	} else {
		var menu state.Menu
		switch menuKind {
		case state.RenameMenuKind:
			menu, err = state.CreateRenameMenu()
		case state.PushWorkoutMenuKind:
			menu, err = state.CreatePushWorkoutMenu()
		case state.DeleteLastWorkoutMenuKind:
			menu, err = state.CreateDeleteLastWorkoutMenu()
		case state.ShowUsersMenuKind:
			menu, err = state.CreateShowUsersMenu()
		default:
			return fmt.Errorf("can't find menu")
		}
		if err != nil {
			return err
		}
		step := menu.Steps[0]
		edit := tgbotapi.NewEditMessageTextAndMarkup(
			fatBotUpdate.Update.FromChat().ID,
			fatBotUpdate.Update.CallbackQuery.Message.MessageID,
			step.Message,
			step.Keyboard,
		)
		fatBotUpdate.Bot.Send(edit)
		state.CreateStateEntry(fatBotUpdate.Update.FromChat().ID, menu.Name)
	}
	return nil
}

func handleCallbacks(fatBotUpdate FatBotUpdate) error {
	err := handleStatefulCallback(fatBotUpdate)
	if err != nil {
		log.Error(err)
	}
	if strings.Contains(fatBotUpdate.Update.CallbackQuery.Message.Text, "rejoin his group do you approve") {
		if err := handleRejoinCallback(fatBotUpdate); err != nil {
			return err
		}
	} else if strings.Contains(fatBotUpdate.Update.CallbackQuery.Message.Text, "new and wants to join a group") {
		if err := handleNewJoinCallback(fatBotUpdate); err != nil {
			return err
		}
	}
	return nil
}

func handleRejoinCallback(fatBotUpdate FatBotUpdate) error {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.CallbackQuery.Message.Chat.ID, "")
	if fatBotUpdate.Update.CallbackQuery.Data == "false" {
		msg.Text = "Declined the request"
	} else {
		userId, _ := strconv.ParseInt(fatBotUpdate.Update.CallbackData(), 10, 64)
		user, err := users.GetUser(uint(userId))
		if err != nil {
			return err
		}
		if err := user.UnBan(fatBotUpdate.Bot); err != nil {
			return fmt.Errorf("Issue with unbanning %s: %s", user.GetName(), err)
		}
		if err := user.InviteExistingUser(fatBotUpdate.Bot); err != nil {
			return fmt.Errorf("Issue with inviting %s: %s", user.GetName(), err)
		}
		if err := user.UpdateActive(true); err != nil {
			return fmt.Errorf("Issue updating active %s: %s", user.GetName(), err)
		}
		if err := user.UpdateOnProbation(true); err != nil {
			return fmt.Errorf("Issue updating probation %s: %s", user.GetName(), err)
		}
		msg.Text = "Ok, approved"
	}
	_, err := fatBotUpdate.Bot.Send(msg)
	return err
}

func handleNewJoinCallback(fatBotUpdate FatBotUpdate) error {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.CallbackQuery.Message.Chat.ID, "")
	dataSlice := strings.Split(fatBotUpdate.Update.CallbackData(), " ")
	userId, _ := strconv.ParseInt(dataSlice[1], 10, 64)
	if dataSlice[0] == "block" {
		msg.Text = "Blocked"
		if err := users.BlockUserId(userId); err != nil {
			log.Error(err)
		}
	} else {
		chatId, _ := strconv.ParseInt(dataSlice[0], 10, 64)
		name := dataSlice[2]
		username := dataSlice[3]
		group, err := users.GetGroup(chatId)
		if err != nil {
			return err
		}
		user := users.User{
			Username:       username,
			Name:           name,
			TelegramUserID: userId,
			Active:         true,
			Groups: []*users.Group{
				&group,
			},
		}
		if err := user.InviteNewUser(fatBotUpdate.Bot, chatId); err != nil {
			log.Error(fmt.Errorf("Issue with inviting: %s", err))
		}
		msg.Text = "Invitation sent"
	}
	_, err := fatBotUpdate.Bot.Send(msg)
	return err
}
