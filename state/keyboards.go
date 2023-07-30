package state

import (
	"fatbot/users"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func createGroupsKeyboard(adminUserId int64) tgbotapi.InlineKeyboardMarkup {
	var groups []users.Group
	switch adminUserId {
	case 0:
		groups = users.GetGroups()
	default:
		groups = users.GetManagedGroups(adminUserId)
	}
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
	backRow := []tgbotapi.InlineKeyboardButton{}
	backButton := tgbotapi.NewInlineKeyboardButtonData("<- Back", "adminmenuback")
	backRow = append(backRow, backButton)
	rows = append(rows, backRow)
	var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}

func createUsersKeyboard(chatId int64, active bool) tgbotapi.InlineKeyboardMarkup {
	var usersList []users.User
	if active {
		usersList = users.GetUsers(chatId)
	} else {
		usersList = users.GetInactiveUsers(chatId)

	}
	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, user := range usersList {
		userLabel := fmt.Sprintf("%s", user.GetName())
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(userLabel, fmt.Sprint(user.TelegramUserID)))
		if len(row) == 3 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 && len(row) < 3 {
		rows = append(rows, row)
	}
	backRow := []tgbotapi.InlineKeyboardButton{}
	backButton := tgbotapi.NewInlineKeyboardButtonData("<- Back", "adminmenuback")
	backRow = append(backRow, backButton)
	rows = append(rows, backRow)
	var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}

func CreateAdminKeyboard(superAdmin bool) tgbotapi.InlineKeyboardMarkup {
	var rename RenameMenu
	var pushWorkout PushWorkoutMenu
	var deleteLastWorkout DeleteLastWorkoutMenu
	var showUsers ShowUsersMenu
	var showEvents ShowEventsMenu
	var rejoinUser RejoinUserMenu
	var banUser BanUserMenu
	var groupLink GroupLinkMenu
	var addGroupAdmin AddGroupAdminMenu
	var menus = []MenuBase{
		rename.CreateMenu(0),
		pushWorkout.CreateMenu(0),
		deleteLastWorkout.CreateMenu(0),
		showUsers.CreateMenu(0),
		showEvents.CreateMenu(0),
		rejoinUser.CreateMenu(0),
		banUser.CreateMenu(0),
		groupLink.CreateMenu(0),
		addGroupAdmin.CreateMenu(0),
	}

	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, menu := range menus {
		if menu.SuperAdminOnly && !superAdmin {
			continue
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			menu.Label,
			menu.Name,
		))
		if len(row) == 2 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	rows = append(rows, row)
	var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}
