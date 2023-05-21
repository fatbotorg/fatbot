package state

import (
	"fatbot/users"
	"fmt"
	"strconv"
	"strings"
)

type State struct {
	ChatId int64
	Value  string
	Kind   menuKind
	Menu   Menu
}

func New(chatId int64) (*State, error) {
	var state State
	var err error
	state.ChatId = chatId
	if state.Value, err = getState(chatId); err != nil {
		return nil, err
	}
	if err = state.setKind(); err != nil {
		return nil, err
	}
	if err := state.setMenu(); err != nil {
		return nil, err
	}
	return &state, err
}

func (state *State) setMenu() (err error) {
	stateSlice := strings.Split(state.Value, ":")
	if len(stateSlice) == 0 {
		return fmt.Errorf("Empty value slice")
	}
	switch state.Kind {
	case RenameMenuKind:
		if state.Menu, err = CreateRenameMenu(); err != nil {
			return err
		}
	case PushWorkoutMenuKind:
		if state.Menu, err = CreatePushWorkoutMenu(); err != nil {
			return err
		}
	}
	return nil
}

func (state *State) IsLastStep() bool {
	return len(state.Menu.Steps) == len(strings.Split(state.Value, ":"))
}

func (state *State) PerformAction(data string) error {
	switch state.Kind {
	case RenameMenuKind:
		telegramUserId, err := state.getTelegramUserId()
		if err != nil {
			return err
		}
		if user, err := users.GetUserById(telegramUserId); err != nil {
			return err
		} else {
			user.Rename(data)
		}
	case PushWorkoutMenuKind:
		telegramUserId, err := state.getTelegramUserId()
		groupChatId, err := state.getGroupChatId()
		if err != nil {
			return err
		}
		if user, err := users.GetUserById(telegramUserId); err != nil {
			return err
		} else {
			days, err := strconv.ParseInt(data, 10, 64)
			if err != nil {
				return err
			}
			if err := user.PushWorkout(days, groupChatId); err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("could not find action")
	}
	return nil
}

func (state *State) getTelegramUserId() (userId int64, err error) {
	for stepIndex, step := range state.Menu.Steps {
		if step.Result == TelegramUserIdStepResult {
			stateSlice := strings.Split(state.Value, ":")
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
	for stepIndex, step := range state.Menu.Steps {
		if step.Result == GroupIdStepResult {
			stateSlice := strings.Split(state.Value, ":")
			groupId, err := strconv.ParseInt(stateSlice[stepIndex+1], 10, 64)
			if err != nil {
				return 0, err
			}
			return groupId, nil
		}
	}
	return 0, fmt.Errorf("Could not find telegramuserid step")
}

//	func (state *State) getUserNewName() (string, error) {
//		for stepIndex, step := range state.Menu.Steps {
//			if step.Result == NewNameStepResult {
//				stateSlice := strings.Split(state.Value, ":")
//				return stateSlice[stepIndex+1], nil
//			}
//		}
//		return "", fmt.Errorf("Could not find newname step")
//	}
func (state *State) ExtractData() (data int64, err error) {
	stateSlice := strings.Split(state.Value, ":")
	return strconv.ParseInt(stateSlice[len(stateSlice)-1], 10, 64)
}

func (state *State) CurrentStep() Step {
	stateSlice := strings.Split(state.Value, ":")
	return state.Menu.Steps[len(stateSlice)]
}

func (state *State) setKind() error {
	stateSlice := strings.Split(state.Value, ":")
	rawKind := stateSlice[0]
	switch rawKind {
	case string(RenameMenuKind):
		state.Kind = RenameMenuKind
		return nil
	case string(PushWorkoutMenuKind):
		state.Kind = PushWorkoutMenuKind
		return nil
	}
	return fmt.Errorf("can't find menu kind")
}

func CreateStateEntry(chatId int64, value string) error {
	if err := set(fmt.Sprint(chatId), value); err != nil {
		return err
	}
	return nil
}

func DeleteStateEntry(chatId int64) error {
	if err := clear(fmt.Sprint(chatId)); err != nil {
		return err
	}
	return nil
}

func getState(chatId int64) (string, error) {
	return get(fmt.Sprint(chatId))
}
