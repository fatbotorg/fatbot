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
	var init bool
	chatId := fatBotUpdate.Update.FromChat().ID
	msg := tgbotapi.NewMessage(chatId, "")
	if fatBotUpdate.Update.CallbackQuery == nil {
		data = fatBotUpdate.Update.Message.Text
		messageId = fatBotUpdate.Update.Message.MessageID
		msg.Text = data
	} else {
		data = fatBotUpdate.Update.CallbackData()
		messageId = fatBotUpdate.Update.CallbackQuery.Message.MessageID
	}
	if !state.HasState(chatId) {
		state.CreateStateEntry(chatId, data)
		init = true
	}
	menuState, err := state.New(chatId)
	if err != nil {
		return err
	}

	if data == "adminmenuback" {
		if menuState.IsFirstStep() {
			edit := tgbotapi.NewEditMessageTextAndMarkup(
				chatId, messageId, "Choose an option", state.CreateAdminKeyboard(),
			)
			if err := state.DeleteStateEntry(chatId); err != nil {
				log.Errorf("Error clearing state: %s", err)
			}
			fatBotUpdate.Bot.Request(edit)
			return
		} else {
			newData, err := state.StepBack(chatId)
			if err != nil {
				return err
			}
			fatBotUpdate.Update.CallbackQuery.Data = newData
			return handleStatefulCallback(fatBotUpdate)
		}
	}
	menu, err := menuState.GetStateMenu()
	if err != nil {
		return err
	}
	if init {
		if err := initAdminMenuFlow(fatBotUpdate, menu); err != nil {
			return err
		}
		return nil
	}
	menuState.Menu = menu
	if err != nil {
		return err
	} else if menu == nil {
		return fmt.Errorf("menu is nil!")
	}
	// NOTE: here ->
	value := menuState.Value + state.Delimiter + data
	if menuState.IsLastStep() {
		menuState.Value = value
		actionData := state.ActionData{
			Data:   data,
			Update: fatBotUpdate.Update,
			Bot:    fatBotUpdate.Bot,
			State:  menuState,
		}
		if err := menuState.Menu.PerformAction(actionData); err != nil {
			log.Error(err)
		} else {
			edit := tgbotapi.NewEditMessageTextAndMarkup(
				chatId, messageId, "Choose an option", state.CreateAdminKeyboard(),
			)
			fatBotUpdate.Bot.Request(edit)
			if fatBotUpdate.Update.CallbackQuery != nil {
				text := fmt.Sprintf("%s done. -> %s", menuState.Menu.CreateMenu().Name, data)
				callback := tgbotapi.NewCallback(fatBotUpdate.Update.CallbackQuery.ID, text)
				fatBotUpdate.Bot.Request(callback)
			} else {
				msg.Text = "Done."
				fatBotUpdate.Bot.Request(msg)
			}
			err := state.DeleteStateEntry(chatId)
			if err != nil {
				log.Error(err)
			}
		}
		return nil
	}
	if err := state.CreateStateEntry(chatId, value); err != nil {
		log.Error(err)
	}
	menuState.Value = value
	step := menuState.CurrentStep()
	switch step.Kind {
	case state.KeyboardStepKind:
		if len(step.Keyboard.InlineKeyboard) == 0 {
			menuState.Value = value
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
	return nil
}

func initAdminMenuFlow(fatBotUpdate FatBotUpdate, menu state.Menu) error {
	if _, err := users.GetUserById(fatBotUpdate.Update.FromChat().ID); err != nil {
		return err
	} else {
		menu := menu.CreateMenu()
		step := menu.Steps[0]
		edit := tgbotapi.NewEditMessageTextAndMarkup(
			fatBotUpdate.Update.FromChat().ID,
			fatBotUpdate.Update.CallbackQuery.Message.MessageID,
			step.Message,
			step.Keyboard,
		)
		fatBotUpdate.Bot.Send(edit)
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
