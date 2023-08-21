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
	ParentMenu     bool
}

type stepKind string
type stepResult string

const (
	Delimiter                 = ":"
	InputStepKind    stepKind = "input"
	KeyboardStepKind stepKind = "keyboard"

	GroupIdStepResult                stepResult = "groupId"
	TelegramUserIdStepResult         stepResult = "telegramUserId"
	TelegramInactiveUserIdStepResult stepResult = "telegramInactiveUserId"
	NewNameStepResult                stepResult = "newName"
	PushDaysStepResult               stepResult = "pushDays"
	OptionResult                     stepResult = "option"
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
type RejoinUserMenu struct {
	MenuBase
}
type BanUserMenu struct {
	MenuBase
}
type GroupLinkMenu struct {
	MenuBase
}
type ManageAdminsMenu struct {
	MenuBase
}
type ShowAdminsMenu struct {
	MenuBase
}
type ChangeAdminsMenu struct {
	MenuBase
}

type Menu interface {
	CreateMenu(userId int64) MenuBase
	PerformAction(ActionData) error
}

var menuMap = map[string]Menu{
	"rename":            RenameMenu{},
	"pushworkout":       PushWorkoutMenu{},
	"deletelastworkout": DeleteLastWorkoutMenu{},
	"showusers":         ShowUsersMenu{},
	"rejoinuser":        RejoinUserMenu{},
	"banuser":           BanUserMenu{},
	"grouplink":         GroupLinkMenu{},
	"adminoptions":      ManageAdminsMenu{},
	"showadmins":        ShowAdminsMenu{},
	"editadmins":        ChangeAdminsMenu{},
}

func (menu ManageAdminsMenu) CreateMenu(userId int64) MenuBase {
	themenu := MenuBase{
		Name:           "adminoptions",
		Label:          "Manage Admins",
		Steps:          []Step{chooseAdminMenuOption},
		SuperAdminOnly: true,
		ParentMenu:     true,
	}
	return themenu
}

func (menu ShowAdminsMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(0)
	return MenuBase{
		Name:           "showadmins",
		Label:          "Show Admins",
		Steps:          []Step{chooseAdminMenuOption, chooseGroup},
		SuperAdminOnly: true,
	}
}

func (menu ChangeAdminsMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	themenu := MenuBase{
		Name:           "editadmins",
		Label:          "Manage Admins",
		Steps:          []Step{chooseAdminMenuOption, chooseAdminEditOption, chooseGroup, userStep},
		SuperAdminOnly: true,
	}
	return themenu
}

func (menu GroupLinkMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	return MenuBase{
		Name:  "grouplink",
		Label: "Group Link",
		Steps: []Step{chooseGroup},
	}
}

func (menu BanUserMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	return MenuBase{
		Name:  "banuser",
		Label: "Ban User",
		Steps: []Step{chooseGroup, userStep},
	}
}

func (menu RejoinUserMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	inactiveUsersStep := userStep
	inactiveUsersStep.Result = TelegramInactiveUserIdStepResult
	themenu := MenuBase{
		Name:  "rejoinuser",
		Label: "Rejoin User",
		Steps: []Step{chooseGroup, inactiveUsersStep},
	}
	return themenu
}

func (menu RenameMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	insertName := Step{
		Name:     "insertname",
		Kind:     InputStepKind,
		Message:  "Insert Name",
		Keyboard: tgbotapi.InlineKeyboardMarkup{},
		Result:   NewNameStepResult,
	}
	themenu := MenuBase{
		Name:  "rename",
		Label: "Rename User",
		Steps: []Step{chooseGroup, userStep, insertName},
	}
	return themenu
}

func (menu PushWorkoutMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	insertDays := Step{
		Name:    "insertdays",
		Kind:    InputStepKind,
		Message: "Insert Days",
		Result:  PushDaysStepResult,
	}
	return MenuBase{
		Name:  "pushworkout",
		Label: "Push Workout",
		Steps: []Step{chooseGroup, userStep, insertDays},
	}
}

func (menu DeleteLastWorkoutMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	return MenuBase{
		Name:  "deletelastworkout",
		Label: "Delete Workout",
		Steps: []Step{chooseGroup, userStep},
	}
}

func (menu ShowUsersMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	return MenuBase{
		Name:  "showusers",
		Label: "Show Users",
		Steps: []Step{chooseGroup},
	}
}

func (step *Step) PopulateKeyboard(data int64) {
	switch step.Result {
	case TelegramUserIdStepResult:
		step.Keyboard = createUsersKeyboard(data, true)
	case TelegramInactiveUserIdStepResult:
		step.Keyboard = createUsersKeyboard(data, false)
	default:
		log.Errorf("unknown step result for keyboard population: %s", step.Result)
	}
}
