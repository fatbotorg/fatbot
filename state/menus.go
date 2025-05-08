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
type RemoveUserMenu struct {
	MenuBase
}
type UpdateRanksMenu struct {
	MenuBase
}
type ManageImmunityMenu struct {
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
	"removeuser":        RemoveUserMenu{},
	"updateranks":       UpdateRanksMenu{},
	"manageimmunity":    ManageImmunityMenu{},
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

func (menu RemoveUserMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)

	// Create a custom step for remove user that shows both active and inactive users
	allUsersStep := userStep
	allUsersStep.Name = "chooseanyuser"
	allUsersStep.Message = "Choose User (Active or Inactive)"

	confirmStep := Step{
		Name:     "confirm",
		Kind:     KeyboardStepKind,
		Message:  "Are you sure you want to remove this user? This action cannot be undone.",
		Keyboard: createConfirmationKeyboard(),
		Result:   OptionResult,
	}

	themenu := MenuBase{
		Name:  "removeuser",
		Label: "Remove User",
		Steps: []Step{chooseGroup, allUsersStep, confirmStep},
	}
	return themenu
}

func (menu UpdateRanksMenu) CreateMenu(userId int64) MenuBase {
	confirmStep := Step{
		Name:     "confirm",
		Kind:     KeyboardStepKind,
		Message:  "Are you sure you want to update all ranks? This may take a while.",
		Keyboard: createConfirmationKeyboard(),
		Result:   OptionResult,
	}
	return MenuBase{
		Name:           "updateranks",
		Label:          "Update All Ranks",
		Steps:          []Step{confirmStep},
		SuperAdminOnly: true,
	}
}

func (menu ManageImmunityMenu) CreateMenu(userId int64) MenuBase {
	chooseGroup := groupStepBase
	chooseGroup.Keyboard = createGroupsKeyboard(userId)
	return MenuBase{
		Name:           "manageimmunity",
		Label:          "Manage User Immunity",
		Steps:          []Step{chooseGroup, userStep},
		SuperAdminOnly: true,
	}
}

func (step *Step) PopulateKeyboard(data int64) {
	switch step.Result {
	case TelegramUserIdStepResult:
		if step.Name == "chooseanyuser" {
			// For removeuser menu, show both active and inactive users
			step.Keyboard = createAllUsersKeyboard(data)
		} else {
			step.Keyboard = createUsersKeyboard(data, true)
		}
	case TelegramInactiveUserIdStepResult:
		step.Keyboard = createUsersKeyboard(data, false)
	default:
		log.Errorf("unknown step result for keyboard population: %s", step.Result)
	}
}
