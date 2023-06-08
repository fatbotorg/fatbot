package updates

import (
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func answerCallback(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
	if _, err := bot.Request(callback); err != nil {
		return err
	}
	return nil
}

func handleStatefulCallback(fatBotUpdate FatBotUpdate) (err error) {
	var data string
	var init bool
	chatId := fatBotUpdate.Update.FromChat().ID
	msg := tgbotapi.NewMessage(chatId, "")
	if fatBotUpdate.Update.CallbackQuery == nil {
		data = fatBotUpdate.Update.Message.Text
		msg.Text = data
	} else {
		data = fatBotUpdate.Update.CallbackData()
		if err := answerCallback(fatBotUpdate); err != nil {
			fullErr := fmt.Errorf("cannot respond to callback with data: %s", err)
			log.Error(fullErr)
			sentry.CaptureException(fullErr)
		}
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
		return handleAdminMenuBackClick(fatBotUpdate, *menuState)
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
	value := menuState.Value + state.Delimiter + data
	if menuState.IsLastStep() {
		return handleAdminMenuLastStep(fatBotUpdate, menuState)
	}
	if err := state.CreateStateEntry(chatId, value); err != nil {
		sentry.CaptureException(err)
		log.Error(err)
	}
	menuState.Value = value
	if err := handleAdminMenuStep(fatBotUpdate, menuState); err != nil {
		return err
	}
	return nil
}

func handleAdminMenuBackClick(fatBotUpdate FatBotUpdate, menuState state.State) error {
	chatId := fatBotUpdate.Update.FromChat().ID
	var messageId int
	if fatBotUpdate.Update.CallbackQuery == nil {
		messageId = fatBotUpdate.Update.Message.MessageID
	} else {
		messageId = fatBotUpdate.Update.CallbackQuery.Message.MessageID
	}
	if menuState.IsFirstStep() {
		edit := tgbotapi.NewEditMessageTextAndMarkup(
			chatId, messageId, "Choose an option", state.CreateAdminKeyboard(),
		)
		if err := state.DeleteStateEntry(chatId); err != nil {
			sentry.CaptureException(err)
			log.Errorf("Error clearing state: %s", err)
		}
		fatBotUpdate.Bot.Request(edit)
		return nil
	} else {
		newData, err := state.StepBack(chatId)
		if err != nil {
			if err := state.DeleteStateEntry(chatId); err != nil {
				sentry.CaptureException(err)
				log.Errorf("Error clearing state: %s", err)
			}
			return err
		}
		fatBotUpdate.Update.CallbackQuery.Data = newData
		return handleStatefulCallback(fatBotUpdate)
	}
}

func handleAdminMenuLastStep(fatBotUpdate FatBotUpdate, menuState *state.State) error {
	var data string
	var messageId int
	chatId := fatBotUpdate.Update.FromChat().ID
	if fatBotUpdate.Update.CallbackQuery == nil {
		messageId = fatBotUpdate.Update.Message.MessageID
	} else {
		messageId = fatBotUpdate.Update.CallbackQuery.Message.MessageID
	}
	msg := tgbotapi.NewMessage(chatId, "")
	if fatBotUpdate.Update.CallbackQuery == nil {
		data = fatBotUpdate.Update.Message.Text
	} else {
		data = fatBotUpdate.Update.CallbackData()
	}
	value := menuState.Value + state.Delimiter + data
	menuState.Value = value
	actionData := state.ActionData{
		Data:   data,
		Update: fatBotUpdate.Update,
		Bot:    fatBotUpdate.Bot,
		State:  menuState,
	}
	if err := menuState.Menu.PerformAction(actionData); err != nil {
		sentry.CaptureException(err)
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
			sentry.CaptureException(err)
			log.Error(err)
		}
	}
	return nil
}

func handleAdminMenuStep(fatBotUpdate FatBotUpdate, menuState *state.State) error {
	var data string
	chatId := fatBotUpdate.Update.FromChat().ID
	messageId := fatBotUpdate.Update.CallbackQuery.Message.MessageID
	msg := tgbotapi.NewMessage(chatId, "")
	step := menuState.CurrentStep()
	if fatBotUpdate.Update.CallbackQuery == nil {
		data = fatBotUpdate.Update.Message.Text
	} else {
		data = fatBotUpdate.Update.CallbackData()
	}
	value := menuState.Value + state.Delimiter + data
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
		sentry.CaptureException(err)
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
		fatBotUpdate.Bot.Send(msg)
	} else {
		userId, _ := strconv.ParseInt(fatBotUpdate.Update.CallbackData(), 10, 64)
		user, err := users.GetUser(uint(userId))
		if err != nil {
			return err
		}
		err = user.Rejoin(fatBotUpdate.Update, fatBotUpdate.Bot)
		if err != nil {
			return err
		}
	}
	return nil
}

func handleNewJoinCallback(fatBotUpdate FatBotUpdate) error {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.CallbackQuery.Message.Chat.ID, "")
	dataSlice := strings.Split(fatBotUpdate.Update.CallbackData(), " ")
	userId, _ := strconv.ParseInt(dataSlice[1], 10, 64)
	if dataSlice[0] == "block" {
		msg.Text = "Blocked"
		if err := users.BlockUserId(userId); err != nil {
			log.Error(err)
			sentry.CaptureException(err)
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
			sentry.CaptureException(err)
		}
		msg.Text = "Invitation sent"
	}
	_, err := fatBotUpdate.Bot.Send(msg)
	return err
}
