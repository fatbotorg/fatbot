package state

import (
	"fmt"
	"strconv"
	"strings"
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
		return nil, err
	}
	return &state, err
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
	default:
		return menu, fmt.Errorf("unknown menu: %s", rawMenu)
	}
	return
}

func (state *State) getTelegramUserId() (userId int64, err error) {
	for stepIndex, step := range state.Menu.CreateMenu().Steps {
		if step.Result == TelegramUserIdStepResult {
			stateSlice := state.getValueSplit()
			userId, err := strconv.ParseInt(stateSlice[stepIndex+1], 10, 64)
			if err != nil {
				return 0, err
			}
			return userId, nil
		}
	}
	return 0, fmt.Errorf("Could not find telegramuserid step")
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
	return 0, fmt.Errorf("Could not find telegramuserid step")
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
	if _, err := getState(chatId); err != nil {
		return false
	}
	return true
}

func getState(chatId int64) (string, error) {
	return get(fmt.Sprint(chatId))
}

func StepBack(chatId int64) (string, error) {
	var iterations int
	var value string
	var err error
	for value, err = get(fmt.Sprint(chatId)); err != nil; iterations++ {
		log.Debug("Looping", "iter", iterations)
		if iterations == 3 {
			return "", err
		}
		value, err = get(fmt.Sprint(chatId))
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
