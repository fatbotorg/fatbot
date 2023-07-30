package state

import (
	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MenuBase struct {
	Name           string
	Label          string
	Steps          []Step
	SuperAdminOnly bool
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
type GroupLinkMenu struct {
	MenuBase
}
type AddGroupAdminMenu struct {
	MenuBase
}

type Menu interface {
	CreateMenu(userId int64) MenuBase
	PerformAction(ActionData) error
}

func (menu AddGroupAdminMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(0),
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
		Name:           "addgroupadmin",
		Label:          "Add Admin",
		Steps:          []Step{chooseGroup, chooseUser},
		SuperAdminOnly: true,
	}
	return themenu
}

func (menu GroupLinkMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(userId),
		Result:   GroupIdStepResult,
	}
	themenu := MenuBase{
		Name:           "grouplink",
		Label:          "Group Link",
		Steps:          []Step{chooseGroup},
		SuperAdminOnly: false,
	}
	return themenu
}

func (menu BanUserMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(userId),
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
		Name:           "banuser",
		Label:          "Ban User",
		Steps:          []Step{chooseGroup, chooseUser},
		SuperAdminOnly: false,
	}
	return themenu
}

func (menu RejoinUserMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(userId),
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
		Name:           "rejoinuser",
		Label:          "Rejoin User",
		Steps:          []Step{chooseGroup, chooseUser},
		SuperAdminOnly: false,
	}
	return themenu
}

func (menu ShowEventsMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(userId),
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
		Name:           "showevents",
		Label:          "Show Events",
		Steps:          []Step{chooseGroup, chooseUser},
		SuperAdminOnly: true,
	}
	return themenu
}

func (menu RenameMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(userId),
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
		Name:           "rename",
		Label:          "Rename User",
		Steps:          []Step{chooseGroup, chooseUser, insertName},
		SuperAdminOnly: false,
	}
	return themenu
}

func (menu PushWorkoutMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(userId),
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
		Name:           "pushworkout",
		Label:          "Push Workout",
		Steps:          []Step{chooseGroup, chooseUser, insertDays},
		SuperAdminOnly: false,
	}
	return themenu
}

func (menu DeleteLastWorkoutMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(userId),
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
		Name:           "deletelastworkout",
		Label:          "Delete Workout",
		Steps:          []Step{chooseGroup, chooseUser},
		SuperAdminOnly: false,
	}
	return themenu
}

func (menu ShowUsersMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := Step{
		Name:     "choosegroup",
		Kind:     KeyboardStepKind,
		Message:  "Choose Group",
		Keyboard: createGroupsKeyboard(userId),
		Result:   GroupIdStepResult,
	}
	themenu := MenuBase{
		Name:           "showusers",
		Label:          "Show Users",
		Steps:          []Step{chooseGroup},
		SuperAdminOnly: false,
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
