package state

import (
	"fatbot/admin"
	"fatbot/users"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Menu struct {
	Name  string
	Steps []Step
	Kind  MenuKind
}

type stepKind string
type MenuKind string
type stepResult string

const (
	Delimiter                            = ":"
	InputStepKind             stepKind   = "input"
	KeyboardStepKind          stepKind   = "keyboard"
	RenameMenuKind            MenuKind   = "rename"
	PushWorkoutMenuKind       MenuKind   = "pushworkout"
	DeleteLastWorkoutMenuKind MenuKind   = "deletelastworkout"
	GroupIdStepResult         stepResult = "groupId"
	TelegramUserIdStepResult  stepResult = "telegramUserId"
	NewNameStepResult         stepResult = "newName"
	PushDaysStepResult        stepResult = "pushDays"
)

type Step struct {
	Name           string
	Kind           stepKind
	Message        string
	Keyboard       tgbotapi.InlineKeyboardMarkup
	KeyboardMethod *func(int64)
	Result         stepResult
}

func createGroupsKeyboard() tgbotapi.InlineKeyboardMarkup {
	groups := users.GetGroupsWithUsers()
	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, group := range groups {
		groupLabel := fmt.Sprintf("%s", group.Title)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			groupLabel,
			fmt.Sprintf("%d", group.ChatID),
		))
		if len(row) == 3 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 && len(row) < 3 {
		rows = append(rows, row)
	}
	var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}

func CreateRenameMenu() (Menu, error) {
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
	menu := Menu{
		Name:  "rename",
		Steps: []Step{chooseGroup, chooseUser, insertName},
		Kind:  RenameMenuKind,
	}
	return menu, nil
}

func CreatePushWorkoutMenu() (Menu, error) {
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
	menu := Menu{
		Name:  "pushworkout",
		Kind:  PushWorkoutMenuKind,
		Steps: []Step{chooseGroup, chooseUser, insertDays},
	}
	return menu, nil
}

func CreateDeleteLastWorkoutMenu() (Menu, error) {
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
	menu := Menu{
		Name:  "deletelastworkout",
		Steps: []Step{chooseGroup, chooseUser},
		Kind:  DeleteLastWorkoutMenuKind,
	}
	return menu, nil
}

func (step *Step) PopulateKeyboard(data int64) {
	switch step.Result {
	case TelegramUserIdStepResult:
		step.Keyboard = admin.CreateUsersKeyboard(data)
	}
}
