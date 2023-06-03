package state

import (
	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MenuBase struct {
	Name  string
	Steps []Step
}

type stepKind string
type stepResult string

const (
	Delimiter                                   = ":"
	InputStepKind                    stepKind   = "input"
	KeyboardStepKind                 stepKind   = "keyboard"
	GroupIdStepResult                stepResult = "groupId"
	TelegramUserIdStepResult         stepResult = "telegramUserId"
	TelegramInactiveUserIdStepResult stepResult = "telegramInactiveUserId"
	NewNameStepResult                stepResult = "newName"
	PushDaysStepResult               stepResult = "pushDays"
)

type Step struct {
	Name     string
	Kind     stepKind
	Message  string
	Keyboard tgbotapi.InlineKeyboardMarkup
	Result   stepResult
}

type RenameMenu struct {
	MenuBase
}
type PushWorkoutMenu struct {
	MenuBase
}
type DeleteLastWorkoutMenu struct {
	MenuBase
}
type ShowUsersMenu struct {
	MenuBase
}
type ShowEventsMenu struct {
	MenuBase
}
type RejoinUserMenu struct {
	MenuBase
}
type BanUserMenu struct {
	MenuBase
}

type Menu interface {
	CreateMenu() MenuBase
	PerformAction(ActionData) error
}

func (menu BanUserMenu) CreateMenu() MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(),
		Result:   GroupIdStepResult,
	}
	chooseUser := Step{
		Name:     "chooseuser",
		Kind:     KeyboardStepKind,
		Message:  "Choose User",
		Keyboard: tgbotapi.InlineKeyboardMarkup{},
		Result:   TelegramUserIdStepResult,
	}
	themenu := MenuBase{
		Name:  "banuser",
		Steps: []Step{chooseGroup, chooseUser},
	}
	return themenu
}

func (menu RejoinUserMenu) CreateMenu() MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(),
		Result:   GroupIdStepResult,
	}
	chooseUser := Step{
		Name:     "chooseuser",
		Kind:     KeyboardStepKind,
		Message:  "Choose User",
		Keyboard: tgbotapi.InlineKeyboardMarkup{},
		Result:   TelegramInactiveUserIdStepResult,
	}
	themenu := MenuBase{
		Name:  "rejoinuser",
		Steps: []Step{chooseGroup, chooseUser},
	}
	return themenu
}

func (menu ShowEventsMenu) CreateMenu() MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(),
		Result:   GroupIdStepResult,
	}
	chooseUser := Step{
		Name:     "chooseuser",
		Kind:     KeyboardStepKind,
		Message:  "Choose User",
		Keyboard: tgbotapi.InlineKeyboardMarkup{},
		Result:   TelegramUserIdStepResult,
	}
	themenu := MenuBase{
		Name:  "showevents",
		Steps: []Step{chooseGroup, chooseUser},
	}
	return themenu
}

func (menu RenameMenu) CreateMenu() MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(),
		Result:   GroupIdStepResult,
	}
	chooseUser := Step{
		Name:     "chooseuser",
		Kind:     KeyboardStepKind,
		Message:  "Choose User",
		Keyboard: tgbotapi.InlineKeyboardMarkup{},
		Result:   TelegramUserIdStepResult,
	}
	insertName := Step{
		Name:     "insertname",
		Kind:     InputStepKind,
		Message:  "Insert Name",
		Keyboard: tgbotapi.InlineKeyboardMarkup{},
		Result:   NewNameStepResult,
	}
	themenu := MenuBase{
		Name:  "rename",
		Steps: []Step{chooseGroup, chooseUser, insertName},
	}
	return themenu
}

func (menu PushWorkoutMenu) CreateMenu() MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(),
		Result:   GroupIdStepResult,
	}
	chooseUser := Step{
		Name:     "chooseuser",
		Kind:     KeyboardStepKind,
		Message:  "Choose User",
		Keyboard: tgbotapi.InlineKeyboardMarkup{},
		Result:   TelegramUserIdStepResult,
	}
	insertDays := Step{
		Name:    "insertdays",
		Kind:    InputStepKind,
		Message: "Insert Days",
		Result:  PushDaysStepResult,
	}
	themenu := MenuBase{
		Name:  "pushworkout",
		Steps: []Step{chooseGroup, chooseUser, insertDays},
	}
	return themenu
}

func (menu DeleteLastWorkoutMenu) CreateMenu() MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(),
		Result:   GroupIdStepResult,
	}
	chooseUser := Step{
		Name:     "chooseuser",
		Kind:     KeyboardStepKind,
		Message:  "Choose User",
		Keyboard: tgbotapi.InlineKeyboardMarkup{},
		Result:   TelegramUserIdStepResult,
	}
	themenu := MenuBase{
		Name:  "deletelastworkout",
		Steps: []Step{chooseGroup, chooseUser},
	}
	return themenu
}

func (menu ShowUsersMenu) CreateMenu() MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(),
		Result:   GroupIdStepResult,
	}
	themenu := MenuBase{
		Name:  "showusers",
		Steps: []Step{chooseGroup},
	}
	return themenu
}

func (step *Step) PopulateKeyboard(data int64) {
	switch step.Result {
	case TelegramUserIdStepResult:
		step.Keyboard = createUsersKeyboard(data, true)
	case TelegramInactiveUserIdStepResult:
		step.Keyboard = createUsersKeyboard(data, false)
	default:
		log.Error("unknown step result for keyboard population")
	}
}
