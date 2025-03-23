package state

import (
	"fatbot/users"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type State struct {
	ChatId int64
	Value  string
	Menu   Menu
	UserId int64
}

func New(chatId int64) (*State, error) {
	var state State
	var err error
	state.ChatId = chatId
	if state.Value, err = getState(chatId); err != nil {
		time.Sleep(1 * time.Second)
		if state.Value, err = getState(chatId); err != nil {
			log.Warn("retrying to get state")
			return nil, err
		}
	}
	return &state, nil
}

func (state *State) getValueSplit() []string {
	return strings.Split(state.Value, Delimiter)
}

func (state *State) IsLastStep() bool {
	return len(state.Menu.CreateMenu(0).Steps) == len(state.getValueSplit())
}

func (state *State) IsFirstStep() bool {
	return len(state.getValueSplit()) == 1
}

func (state *State) GetStateMenu(data string) (menu Menu, err error) {
	if state == nil {
		err = fmt.Errorf("state is nil")
		return
	}
	rawSplit := state.getValueSplit()
	var rawMenu string
	for i, step := range rawSplit {
		if menu, ok := menuMap[step]; ok {
			isParent := menu.CreateMenu(0).ParentMenu
			if !isParent {
				rawMenu = rawSplit[i]
				break
			}
		}
	}
	if _, ok := menuMap[data]; ok {
		rawMenu = data
	}
	var ok bool
	if menu, ok = menuMap[rawMenu]; !ok {
		return menu, fmt.Errorf("no such menu %s", rawMenu)
	}
	return
}

func (state *State) getOption() (option string, err error) {
	steps := state.Menu.CreateMenu(0).Steps
	for stepIndex := len(steps) - 1; stepIndex >= 0; stepIndex-- {
		if steps[stepIndex].Result == OptionResult {
			stateSlice := state.getValueSplit()
			return stateSlice[stepIndex+1], nil
		}
	}
	return "", nil
}

func (state *State) getTelegramUserId() (userId int64, err error) {
	for stepIndex, step := range state.Menu.CreateMenu(0).Steps {
		switch step.Result {
		case TelegramUserIdStepResult, TelegramInactiveUserIdStepResult:
			stateSlice := state.getValueSplit()
			userId, err := strconv.ParseInt(stateSlice[stepIndex+1], 10, 64)
			if err != nil {
				return 0, err
			}
			return userId, nil
		}
	}
	return 0, fmt.Errorf("could not find telegramuserid step")
}

func (state *State) getGroupChatId() (userId int64, err error) {
	for stepIndex, step := range state.Menu.CreateMenu(0).Steps {
		if step.Result == GroupIdStepResult {
			stateSlice := state.getValueSplit()
			groupId, err := strconv.ParseInt(stateSlice[stepIndex+1], 10, 64)
			if err != nil {
				return 0, err
			}
			return groupId, nil
		}
	}
	return 0, fmt.Errorf("could not find groupchatid step")
}

func (state *State) getGroupId() (groupId int64, err error) {
	// This is an alias for getGroupChatId for clarity in usage
	return state.getGroupChatId()
}

func (state *State) ExtractData() (data int64, err error) {
	stateSlice := state.getValueSplit()
	return strconv.ParseInt(stateSlice[len(stateSlice)-1], 10, 64)
}

func (state *State) CurrentStep() Step {
	stateSlice := state.getValueSplit()
	return state.Menu.CreateMenu(0).Steps[len(stateSlice)-1]
}

func CreateStateEntry(chatId int64, value string) error {
	if err := set(fmt.Sprint(chatId), value); err != nil {
		return err
	}
	return nil
}

func DeleteStateEntry(chatId int64) error {
	if err := clear(chatId); err != nil {
		return err
	}
	return nil
}

func HasState(chatId int64) bool {
	if state, err := getState(chatId); err != nil || state == "" {
		return false
	}
	return true
}

func getState(chatId int64) (string, error) {
	return get(fmt.Sprint(chatId))
}

func StepBack(chatId int64) (string, error) {
	value, err := get(fmt.Sprint(chatId))
	if err != nil {
		return "", err
	}
	stateSlice := strings.Split(value, ":")
	stateSlice = stateSlice[:len(stateSlice)-1]
	newValue := strings.Join(stateSlice, ":")
	if err := set(fmt.Sprint(chatId), newValue); err != nil {
		return "", err
	}
	if err := DeleteStateEntry(chatId); err != nil {
		return "", err
	}
	return stateSlice[len(stateSlice)-1], nil
}

func HandleAdminCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	if err := DeleteStateEntry(update.FromChat().ID); err != nil {
		log.Errorf("Error clearing state: %s", err)
	}
	msg := tgbotapi.NewMessage(update.FromChat().ID, "Choose an option")
	userId := update.SentFrom().ID
	user, err := users.GetUserById(userId)
	if err != nil {
		log.Error(err)
		return msg
	}
	var adminKeyboard tgbotapi.InlineKeyboardMarkup
	adminKeyboard = CreateAdminKeyboard(user.IsAdmin)
	msg.ReplyMarkup = adminKeyboard
	return msg
}
