package state

import (
	"fatbot/users"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func createAdminManagementMenu() tgbotapi.InlineKeyboardMarkup {
	var adminKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("List", "showadmins"),
			tgbotapi.NewInlineKeyboardButtonData("Edit", "editadmins"),
		),
	)
	return adminKeyboard
}

func createAdminManagementEditMenu() tgbotapi.InlineKeyboardMarkup {
	var adminKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Add Admin", "addadmin"),
			tgbotapi.NewInlineKeyboardButtonData("Remove Admin", "removeadmin"),
		),
	)
	return adminKeyboard
}

func createGroupsKeyboard(adminUserId int64) tgbotapi.InlineKeyboardMarkup {
	var groupsInfo []map[string]interface{}

	// When showing groups for removal, always show all groups with user counts
	if adminUserId == 0 {
		groupsInfo = users.GetGroupsWithUserCounts()
	} else {
		// For local admin operations, filter by managed groups
		var groups []users.Group = users.GetManagedGroups(adminUserId)
		for _, group := range groups {
			groupsInfo = append(groupsInfo, map[string]interface{}{
				"group": group,
			})
		}
	}

	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}

	for _, info := range groupsInfo {
		group := info["group"].(users.Group)

		// Create label with user counts if available
		var groupLabel string
		if info["totalCount"] != nil {
			groupLabel = fmt.Sprintf("%s (%d users)",
				group.Title,
				info["totalCount"].(int))
		} else {
			groupLabel = group.Title
		}

		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			groupLabel,
			fmt.Sprintf("%d", group.ChatID),
		))

		// Only 2 buttons per row for group removal to fit more text
		if len(row) == 2 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}

	if len(row) > 0 {
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

// createAllUsersKeyboard creates a keyboard that includes both active and inactive users
func createAllUsersKeyboard(chatId int64) tgbotapi.InlineKeyboardMarkup {
	// Get both active and inactive users
	activeUsers := users.GetUsers(chatId)
	inactiveUsers := users.GetInactiveUsers(chatId)

	// Combine the lists
	allUsers := append(activeUsers, inactiveUsers...)

	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}

	for _, user := range allUsers {
		// Add status indicator to inactive users
		userLabel := user.GetName()
		if !user.Active {
			userLabel = userLabel + " (inactive)"
		}

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

func createConfirmationKeyboard() tgbotapi.InlineKeyboardMarkup {
	var keyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Yes", "yes"),
			tgbotapi.NewInlineKeyboardButtonData("No", "no"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("<- Back", "adminmenuback"),
		),
	)
	return keyboard
}

// createRemoveGroupKeyboard creates a specialized keyboard for the group removal menu
// with detailed information about each group and its users
func createRemoveGroupKeyboard() tgbotapi.InlineKeyboardMarkup {
	groupsInfo := users.GetGroupsWithUserCounts()

	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}

	for _, info := range groupsInfo {
		group := info["group"].(users.Group)
		activeCount := info["activeCount"].(int)
		inactiveCount := info["inactiveCount"].(int)
		totalCount := info["totalCount"].(int)

		// Create detailed label with active, inactive and total user counts
		groupLabel := fmt.Sprintf("%s (%d active / %d inactive / %d total)",
			group.Title,
			activeCount,
			inactiveCount,
			totalCount)

		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			groupLabel,
			fmt.Sprintf("%d", group.ChatID),
		))

		// Only 1 button per row for group removal to fit more text
		rows = append(rows, row)
		row = []tgbotapi.InlineKeyboardButton{}
	}

	backRow := []tgbotapi.InlineKeyboardButton{}
	backButton := tgbotapi.NewInlineKeyboardButtonData("<- Back", "adminmenuback")
	backRow = append(backRow, backButton)
	rows = append(rows, backRow)

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func CreateAdminKeyboard(superAdmin bool) tgbotapi.InlineKeyboardMarkup {
	var rename RenameMenu
	var pushWorkout PushWorkoutMenu
	var deleteLastWorkout DeleteLastWorkoutMenu
	var showUsers ShowUsersMenu
	var rejoinUser RejoinUserMenu
	var banUser BanUserMenu
	var groupLink GroupLinkMenu
	var manageAdmins ManageAdminsMenu
	var removeUser RemoveUserMenu
	var removeGroup RemoveGroupMenu
	var menus = []MenuBase{
		rename.CreateMenu(0),
		pushWorkout.CreateMenu(0),
		deleteLastWorkout.CreateMenu(0),
		showUsers.CreateMenu(0),
		rejoinUser.CreateMenu(0),
		banUser.CreateMenu(0),
		groupLink.CreateMenu(0),
		manageAdmins.CreateMenu(0),
		removeUser.CreateMenu(0),
		removeGroup.CreateMenu(0),
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
