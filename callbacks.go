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
	if menuState.IsLastStep() {
		if err := menuState.PerformAction(data, *fatBotUpdate.Bot, fatBotUpdate.Update); err != nil {
			log.Error(err)
		}
		msg.Text = fmt.Sprintf("%s done. -> %s", menuState.Menu.Name, data)
		fatBotUpdate.Bot.Request(msg)
		err := state.DeleteStateEntry(chatId)
		if err != nil {
			log.Error(err)
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
	value := menuState.Value + state.Delimiter + data
	state.CreateStateEntry(chatId, value)
	return nil
}

// func initAdminRename(fatBotUpdate FatBotUpdate) error {
// 	if _, err := users.GetUserById(fatBotUpdate.Update.FromChat().ID); err != nil {
// 		return err
// 	} else {
// 		if menu, err := state.CreateRenameMenu(); err != nil {
// 			return err
// 		} else {
// 			step := menu.Steps[0]
// 			edit := tgbotapi.NewEditMessageTextAndMarkup(
// 				fatBotUpdate.Update.FromChat().ID,
// 				fatBotUpdate.Update.CallbackQuery.Message.MessageID,
// 				step.Message,
// 				step.Keyboard,
// 			)
// 			fatBotUpdate.Bot.Send(edit)
// 			state.CreateStateEntry(fatBotUpdate.Update.FromChat().ID, menu.Name)
// 		}
// 	}
// 	return nil
// }

// func initAdminPushWorkout(fatBotUpdate FatBotUpdate) error {
// 	if _, err := users.GetUserById(fatBotUpdate.Update.FromChat().ID); err != nil {
// 		return err
// 	} else {
// 		if menu, err := state.CreatePushWorkoutMenu(); err != nil {
// 			return err
// 		} else {
// 			step := menu.Steps[0]
// 			edit := tgbotapi.NewEditMessageTextAndMarkup(
// 				fatBotUpdate.Update.FromChat().ID,
// 				fatBotUpdate.Update.CallbackQuery.Message.MessageID,
// 				step.Message,
// 				step.Keyboard,
// 			)
// 			fatBotUpdate.Bot.Send(edit)
// 			state.CreateStateEntry(fatBotUpdate.Update.FromChat().ID, menu.Name)
// 		}
// 	}
// 	return nil
// }

// func initAdminDeleteLastWorkout(fatBotUpdate FatBotUpdate) error {
// 	if _, err := users.GetUserById(fatBotUpdate.Update.FromChat().ID); err != nil {
// 		return err
// 	} else {
// 		if menu, err := state.CreateDeleteLastWorkoutMenu(); err != nil {
// 			return err
// 		} else {
// 			step := menu.Steps[0]
// 			edit := tgbotapi.NewEditMessageTextAndMarkup(
// 				fatBotUpdate.Update.FromChat().ID,
// 				fatBotUpdate.Update.CallbackQuery.Message.MessageID,
// 				step.Message,
// 				step.Keyboard,
// 			)
// 			fatBotUpdate.Bot.Send(edit)
// 			state.CreateStateEntry(fatBotUpdate.Update.FromChat().ID, menu.Name)
// 		}
// 	}
// 	return nil
// }

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
	switch fatBotUpdate.Update.CallbackQuery.Message.Text {
	// case "Pick a user to delete last workout for":
	// 	if err := handleDeleteLastWorkoutCallback(fatBotUpdate); err != nil {
	// 		return err
	// 	}
	// case "Pick a user to rename":
	// 	if err := handleRenameUserCallback(fatBotUpdate); err != nil {
	// 		return err
	// 	}
	// case "Pick a user to change workout for":
	// 	if err := handleChangeWorkoutCallback(fatBotUpdate); err != nil {
	// 		return err
	// 	}
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

// func handleDeleteLastWorkoutCallback(fatBotUpdate FatBotUpdate) error {
// 	msg := tgbotapi.NewMessage(fatBotUpdate.Update.CallbackQuery.Message.Chat.ID, "")
// 	callback := tgbotapi.NewCallback(fatBotUpdate.Update.CallbackQuery.ID, fatBotUpdate.Update.CallbackQuery.Data)
// 	if _, err := fatBotUpdate.Bot.Request(callback); err != nil {
// 		panic(err)
// 	}
// 	userId, _ := strconv.ParseInt(fatBotUpdate.Update.CallbackQuery.Data, 10, 64)
// 	user, err := users.GetUserById(userId)
// 	if err != nil {
// 		return err
// 	}
// 	chatId, err := user.GetChatId()
// 	if err != nil {
// 		return err
// 	}
// 	if newLastWorkout, err := user.RollbackLastWorkout(chatId); err != nil {
// 		return err
// 	} else {
// 		message := fmt.Sprintf("Deleted last workout for user %s\nRolledback to: %s",
// 			user.GetName(), newLastWorkout.CreatedAt.Format("2006-01-02 15:04:05"))
// 		msg.Text = message
// 		if _, err := fatBotUpdate.Bot.Send(msg); err != nil {
// 			return err
// 		}
// 		messageToUser := tgbotapi.NewMessage(0,
// 			fmt.Sprintf("Your last workout was cancelled by the admin.\nUpdated workout: %s",
// 				newLastWorkout.CreatedAt))
// 		user.SendPrivateMessage(fatBotUpdate.Bot, messageToUser)
// 	}
// 	return nil
// }

func handleRenameUserCallback(fatBotUpdate FatBotUpdate) error {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.CallbackQuery.Message.Chat.ID, "")
	callback := tgbotapi.NewCallback(fatBotUpdate.Update.CallbackQuery.ID, fatBotUpdate.Update.CallbackQuery.Data)
	if _, err := fatBotUpdate.Bot.Request(callback); err != nil {
		panic(err)
	}
	msg.Text = fmt.Sprintf("/admin_rename %s newname", fatBotUpdate.Update.CallbackData())
	if _, err := fatBotUpdate.Bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func handleChangeWorkoutCallback(fatBotUpdate FatBotUpdate) error {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.CallbackQuery.Message.Chat.ID, "")
	callback := tgbotapi.NewCallback(fatBotUpdate.Update.CallbackQuery.ID, fatBotUpdate.Update.CallbackQuery.Data)
	if _, err := fatBotUpdate.Bot.Request(callback); err != nil {
		panic(err)
	}
	msg.Text = fmt.Sprintf("/admin_push_workout %s days", fatBotUpdate.Update.CallbackData())
	if _, err := fatBotUpdate.Bot.Send(msg); err != nil {
		return err
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
