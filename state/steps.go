package state

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

var groupStepBase = Step{
	Name:    "choosegroup",
	Kind:    KeyboardStepKind,
	Message: "Choose Group",
	Result:  GroupIdStepResult,
}

var userStep = Step{
	Name:     "chooseuser",
	Kind:     KeyboardStepKind,
	Message:  "Choose User",
	Keyboard: tgbotapi.InlineKeyboardMarkup{},
	Result:   TelegramUserIdStepResult,
}

var chooseAdminMenuOption = Step{
	Name:     "adminoptions",
	Kind:     KeyboardStepKind,
	Message:  "Choose Option",
	Keyboard: createAdminManagementMenu(),
	Result:   OptionResult,
}

var chooseAdminEditOption = Step{
	Name:     "editadmins",
	Kind:     KeyboardStepKind,
	Message:  "Choose Option",
	Keyboard: createAdminManagementEditMenu(),
	Result:   OptionResult,
}
