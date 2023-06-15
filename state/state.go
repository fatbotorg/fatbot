package state

import (
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
	return len(state.Menu.CreateMenu().Steps) == len(state.getValueSplit())
}

func (state *State) IsFirstStep() bool {
	return len(state.getValueSplit()) == 1
}

func (state *State) GetStateMenu() (menu Menu, err error) {
	if state == nil {
		err = fmt.Errorf("state is nil")
		return
	}
	rawSplit := state.getValueSplit()
	rawMenu := rawSplit[0]
	switch rawMenu {
	case "rename":
		menu = &RenameMenu{}
	case "pushworkout":
		menu = &PushWorkoutMenu{}
	case "deletelastworkout":
		menu = &DeleteLastWorkoutMenu{}
	case "showusers":
		menu = &ShowUsersMenu{}
	case "showevents":
		menu = &ShowEventsMenu{}
	case "rejoinuser":
		menu = &RejoinUserMenu{}
	case "banuser":
		menu = &BanUserMenu{}
	case "grouplink":
		menu = &GrouopLinkMenu{}
	default:
		return menu, fmt.Errorf("unknown menu: %s", rawMenu)
	}
	return
}

func (state *State) getTelegramUserId() (userId int64, err error) {
	for stepIndex, step := range state.Menu.CreateMenu().Steps {
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
	for stepIndex, step := range state.Menu.CreateMenu().Steps {
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

func (state *State) ExtractData() (data int64, err error) {
	stateSlice := state.getValueSplit()
	return strconv.ParseInt(stateSlice[len(stateSlice)-1], 10, 64)
}

func (state *State) CurrentStep() Step {
	stateSlice := state.getValueSplit()
	return state.Menu.CreateMenu().Steps[len(stateSlice)-1]
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
	adminKeyboard := CreateAdminKeyboard()
	msg.ReplyMarkup = adminKeyboard
	return msg
}
