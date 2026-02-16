package updates

import (
	"encoding/json"
	"fatbot/ai"
	"fatbot/db"
	"fatbot/garmin"
	"fatbot/notify"
	"fatbot/state"
	"fatbot/users"
	"fatbot/whoop"
	"fmt"
	"strconv"
	"strings"
	"time"

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
		data = strings.ReplaceAll(data, state.Delimiter, " ")
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
		if err := state.CreateStateEntry(chatId, data); err != nil {
			logErr := fmt.Errorf("problem with creating state entry: %d", chatId)
			log.Error(logErr)
			return err
		}
		init = true
	}
	menuState, err := state.New(chatId)
	if err != nil {
		return err
	}
	if data == "adminmenuback" {
		return handleAdminMenuBackClick(fatBotUpdate, *menuState)
	}
	menu, err := menuState.GetStateMenu(data)
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
	adminUser, _ := users.GetUserById(chatId)
	var messageId int
	if fatBotUpdate.Update.CallbackQuery == nil {
		messageId = fatBotUpdate.Update.Message.MessageID
	} else {
		messageId = fatBotUpdate.Update.CallbackQuery.Message.MessageID
	}
	if menuState.IsFirstStep() {
		edit := tgbotapi.NewEditMessageTextAndMarkup(
			chatId, messageId, "Choose an option", state.CreateAdminKeyboard(adminUser.IsAdmin),
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
	chatId := fatBotUpdate.Update.FromChat().ID
	adminUser, _ := users.GetUserById(chatId)
	msg := tgbotapi.NewMessage(chatId, "")
	if fatBotUpdate.Update.CallbackQuery == nil {
		data = fatBotUpdate.Update.Message.Text
		data = strings.ReplaceAll(data, state.Delimiter, " ")
		msg.Text = data
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
		if _, ok := err.(*state.MenuActionDoneError); ok {
			return nil
		}
		sentry.CaptureException(err)
		log.Error(err)
	} else {
		if fatBotUpdate.Update.CallbackQuery != nil {
			messageId := fatBotUpdate.Update.CallbackQuery.Message.MessageID
			edit := tgbotapi.NewEditMessageTextAndMarkup(
				chatId, messageId, "Choose an option", state.CreateAdminKeyboard(adminUser.IsAdmin),
			)
			fatBotUpdate.Bot.Request(edit)
			text := fmt.Sprintf("%s done. -> %s", menuState.Menu.CreateMenu(0).Name, data)
			callback := tgbotapi.NewCallback(fatBotUpdate.Update.CallbackQuery.ID, text)
			fatBotUpdate.Bot.Request(callback)
		} else {
			msg.Text = "Choose an option"
			msg.ReplyMarkup = state.CreateAdminKeyboard(adminUser.IsAdmin)
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
	msg := tgbotapi.NewMessage(chatId, "")
	step := menuState.CurrentStep()
	if fatBotUpdate.Update.CallbackQuery == nil {
		data = fatBotUpdate.Update.Message.Text
		data = strings.ReplaceAll(data, state.Delimiter, " ")
	} else {
		data = fatBotUpdate.Update.CallbackData()
	}
	value := menuState.Value + state.Delimiter + data
	if menuState.Menu.CreateMenu(0).Name == "instaspotlight" && data == "random" {
		actionData := state.ActionData{
			Data:   data,
			Update: fatBotUpdate.Update,
			Bot:    fatBotUpdate.Bot,
			State:  menuState,
		}
		if err := menuState.Menu.PerformAction(actionData); err != nil {
			if _, ok := err.(*state.MenuActionDoneError); !ok {
				sentry.CaptureException(err)
				log.Error(err)
			}
		}
		return nil
	}
	switch step.Kind {
	case state.KeyboardStepKind:
		if len(step.Keyboard.InlineKeyboard) == 0 {
			menuState.Value = value
			data, _ := menuState.ExtractData()
			step.PopulateKeyboard(data)
		}
		messageText := step.Message
		if step.Name == "confirmpsa" {
			rawMessage := data
			stylized := ai.StylizePSA(rawMessage)
			state.SetWithTTL(fmt.Sprintf("psa:stylized:%d", chatId), stylized, 3600)
			messageText = fmt.Sprintf("AI Stylized Preview:\n\n%s", stylized)
		}
		if fatBotUpdate.Update.CallbackQuery != nil {
			messageId := fatBotUpdate.Update.CallbackQuery.Message.MessageID
			edit := tgbotapi.NewEditMessageTextAndMarkup(
				chatId, messageId, messageText, step.Keyboard,
			)
			if step.Name == "confirmpsa" {
				edit.ParseMode = "Markdown"
			}
			if _, err := fatBotUpdate.Bot.Send(edit); err != nil {
				log.Error(err)
			}
		} else {
			msg.Text = messageText
			msg.ReplyMarkup = step.Keyboard
			if step.Name == "confirmpsa" {
				msg.ParseMode = "Markdown"
			}
			if _, err := fatBotUpdate.Bot.Send(msg); err != nil {
				log.Error(err)
			}
		}
	case state.InputStepKind:
		msg.Text = step.Message
		fatBotUpdate.Bot.Request(msg)
	}
	return nil
}

func initAdminMenuFlow(fatBotUpdate FatBotUpdate, menu state.Menu) error {
	if user, err := users.GetUserById(fatBotUpdate.Update.FromChat().ID); err != nil {
		return err
	} else {
		menu := menu.CreateMenu(user.TelegramUserID)
		step := menu.Steps[0]
		if step.Kind == state.InputStepKind {
			msg := tgbotapi.NewMessage(fatBotUpdate.Update.FromChat().ID, step.Message)
			fatBotUpdate.Bot.Send(msg)
		}
		edit := tgbotapi.NewEditMessageTextAndMarkup(
			fatBotUpdate.Update.FromChat().ID,
			fatBotUpdate.Update.CallbackQuery.Message.MessageID,
			step.Message,
			step.Keyboard,
		)
		fatBotUpdate.Bot.Request(edit)
	}
	return nil
}

func handleCallbacks(fatBotUpdate FatBotUpdate) error {
	if strings.Contains(fatBotUpdate.Update.CallbackQuery.Message.Text, "New join request") ||
		strings.Contains(fatBotUpdate.Update.CallbackQuery.Message.Text, "wants to join using a link") {
		if err := handleNewJoinCallback(fatBotUpdate); err != nil {
			return err
		}
	} else if strings.HasPrefix(fatBotUpdate.Update.CallbackData(), "whoop:") {
		if err := handleWhoopBonusCallback(fatBotUpdate); err != nil {
			return err
		}
	} else if strings.HasPrefix(fatBotUpdate.Update.CallbackData(), "garmin:") {
		if err := handleGarminBonusCallback(fatBotUpdate); err != nil {
			return err
		}
	} else {
		err := handleStatefulCallback(fatBotUpdate)
		if err != nil {
			sentry.CaptureException(err)
			log.Error(err)
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
		message, err := handleAdminJoinApproval(fatBotUpdate.Bot, dataSlice, userId)
		if err != nil {
			return err
		}
		msg.Text = message
	}
	_, err := fatBotUpdate.Bot.Send(msg)
	return err
}

func handleWhoopBonusCallback(fatBotUpdate FatBotUpdate) error {
	data := fatBotUpdate.Update.CallbackData()
	parts := strings.Split(data, ":")
	action := parts[1] // yes or no
	whoopID := parts[2]
	bot := fatBotUpdate.Bot
	user, err := users.GetUserById(fatBotUpdate.Update.CallbackQuery.From.ID)
	if err != nil {
		return err
	}

	// Clean up pending state
	state.ClearString("whoop:pending:" + whoopID)

	// Update message to remove buttons
	edit := tgbotapi.NewEditMessageReplyMarkup(
		fatBotUpdate.Update.CallbackQuery.Message.Chat.ID,
		fatBotUpdate.Update.CallbackQuery.Message.MessageID,
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}},
	)
	bot.Request(edit)

	if action == "no" {
		// Mark as ignored
		state.SetWithTTL("whoop:ignored:"+whoopID, "1", 604800) // 7 days

		// Check if it's a bonus (secondary) workout or just a small one
		lastWorkout, err := user.GetLastWorkout()
		accessToken, errToken := user.GetValidWhoopAccessToken()
		if errToken != nil {
			return errToken
		}
		workout, errWorkout := whoop.GetWorkoutById(accessToken, whoopID)
		if errWorkout != nil {
			return errWorkout
		}

		isBonus := err == nil && users.IsSameDay(lastWorkout.CreatedAt, workout.Start)

		if !isBonus {
			bot.Send(tgbotapi.NewMessage(user.TelegramUserID, "Understood. I won't report this workout."))
			return nil
		}

		// Update message to remove buttons
		edit := tgbotapi.NewEditMessageReplyMarkup(
			fatBotUpdate.Update.CallbackQuery.Message.Chat.ID,
			fatBotUpdate.Update.CallbackQuery.Message.MessageID,
			tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}},
		)
		bot.Request(edit)

		duration := workout.End.Sub(workout.Start)

		if err := user.LoadGroups(); err != nil {
			return err
		}
		for _, group := range user.Groups {
			msg := tgbotapi.NewMessage(group.ChatID, fmt.Sprintf(
				"🏃 %s added a bonus activity: %s\n\nStrain: %.1f\nCalories: %.0f\nAvg HR: %d\nDuration: %.0f min\n\n(This activity is not counted as another workout.)",
				user.GetName(),
				workout.SportName,
				workout.Score.Strain,
				workout.Score.Kilojoule/4.184,
				workout.Score.AverageHeartRate,
				duration.Minutes(),
			))
			bot.Send(msg)
		}
		return nil
	}

	if action == "yes" {
		// Process as normal workout
		accessToken, err := user.GetValidWhoopAccessToken()
		if err != nil {
			return err
		}
		record, err := whoop.GetWorkoutById(accessToken, whoopID)
		if err != nil {
			return err
		}
		duration := record.End.Sub(record.Start)

		if err := user.LoadGroups(); err != nil {
			return err
		}

		for _, group := range user.Groups {
			workout := users.Workout{
				UserID:  user.ID,
				GroupID: group.ID,
				WhoopID: record.ID,
			}
			db.DBCon.Create(&workout)
			notify.NotifyWorkout(bot, user, workout, record.SportName, record.Score.Strain, record.Score.Kilojoule/4.184, record.Score.AverageHeartRate, duration.Minutes(), 0, "", "")
		}
	}
	return nil
}

func handleGarminBonusCallback(fatBotUpdate FatBotUpdate) error {
	data := fatBotUpdate.Update.CallbackData()
	parts := strings.Split(data, ":")
	action := parts[1] // yes or no
	summaryID := parts[2]
	bot := fatBotUpdate.Bot
	user, err := users.GetUserById(fatBotUpdate.Update.CallbackQuery.From.ID)
	if err != nil {
		return err
	}

	// Clean up pending state
	state.ClearString("garmin:pending:" + summaryID)

	// Update message to remove buttons
	edit := tgbotapi.NewEditMessageReplyMarkup(
		fatBotUpdate.Update.CallbackQuery.Message.Chat.ID,
		fatBotUpdate.Update.CallbackQuery.Message.MessageID,
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}},
	)
	bot.Request(edit)

	if action == "no" {
		// Mark as ignored
		state.SetWithTTL("garmin:ignored:"+summaryID, "1", 604800) // 7 days
		bot.Send(tgbotapi.NewMessage(user.TelegramUserID, "Understood. I won't report this Garmin activity."))
		return nil
	}

	if action == "yes" {
		// Process as normal workout
		if users.GarminWorkoutExists(summaryID) {
			bot.Send(tgbotapi.NewMessage(user.TelegramUserID, "This workout was already registered automatically."))
			return nil
		}
		activityDataJSON, err := state.Get("garmin:data:" + summaryID)
		if err != nil {
			return fmt.Errorf("could not find Garmin activity data in state: %s", err)
		}
		var record garmin.ActivityData
		if err := json.Unmarshal([]byte(activityDataJSON), &record); err != nil {
			return fmt.Errorf("failed to unmarshal Garmin activity data: %s", err)
		}
		state.ClearString("garmin:data:" + summaryID)

		duration := time.Duration(record.DurationInSeconds) * time.Second

		if err := user.LoadGroups(); err != nil {
			return err
		}

		for _, group := range user.Groups {
			workout := users.Workout{
				UserID:   user.ID,
				GroupID:  group.ID,
				GarminID: record.SummaryID,
			}
			db.DBCon.Create(&workout)
			notify.NotifyWorkout(bot, user, workout, record.ActivityName, 0, record.Calories, record.AverageHeartRate, duration.Minutes(), record.DistanceInMeters, record.DeviceName, record.ActivityType)
		}
	}
	return nil
}

func handleAdminJoinApproval(bot *tgbotapi.BotAPI, dataSlice []string, userId int64) (messageText string, err error) {
	candidate, err := users.GetUserById(userId)
	if err != nil {
		if _, noSuchUserError := err.(*users.NoSuchUserError); noSuchUserError {
			return handleAdminJoinApprovalCreation(bot, dataSlice, userId)
		} else {
			return "", err
		}
	}
	chatId, _ := strconv.ParseInt(dataSlice[0], 10, 64)
	if candidate.IsInGroup(chatId) {
		messageText = "user already exists in this group"
		return messageText, nil
	}
	return handleAdminJoinApprovalCreation(bot, dataSlice, userId)
}

func handleAdminJoinApprovalCreation(bot *tgbotapi.BotAPI, dataSlice []string, userId int64) (messageText string, err error) {
	name := dataSlice[2]
	username := dataSlice[3]
	chatId, _ := strconv.ParseInt(dataSlice[0], 10, 64)
	group, err := users.GetGroup(chatId)
	if err != nil {
		return "", err
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
	if err := user.InviteNewUser(bot, chatId); err != nil {
		log.Error(fmt.Errorf("Issue with inviting: %s", err))
		sentry.CaptureException(err)
	}
	messageText = "Invitation sent"
	return
}
