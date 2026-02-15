package state

import (
	"fatbot/users"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func createAdminManagementMenu() tgbotapi.InlineKeyboardMarkup {
	adminKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("List", "showadmins"),
			tgbotapi.NewInlineKeyboardButtonData("Edit", "editadmins"),
		),
	)
	return adminKeyboard
}

func createAdminManagementEditMenu() tgbotapi.InlineKeyboardMarkup {
	adminKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Add Admin", "addadmin"),
			tgbotapi.NewInlineKeyboardButtonData("Remove Admin", "removeadmin"),
		),
	)
	return adminKeyboard
}

func createGroupsKeyboard(adminUserId int64) tgbotapi.InlineKeyboardMarkup {
	var groups []users.Group

	// Special case: adminUserId == 0 means we explicitly want all groups (used by system or super admin functions)
	if adminUserId == 0 {
		groups = users.GetGroups()
	} else {
		// Check if the user is a super admin
		user, err := users.GetUserById(adminUserId)
		if err == nil && user.IsAdmin {
			// Super admins see all groups
			groups = users.GetGroups()
		} else {
			// Regular admins only see their managed groups
			groups = users.GetManagedGroups(adminUserId)
		}
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
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
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
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}

func createConfirmationKeyboard() tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
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

func createPSAApprovalKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Approve", "approve"),
			tgbotapi.NewInlineKeyboardButtonData("Edit", "edit"),
			tgbotapi.NewInlineKeyboardButtonData("Cancel", "cancel"),
		),
	)
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
	var updateRanks UpdateRanksMenu
	var manageImmunity ManageImmunityMenu
	var disputeWorkout DisputeWorkoutMenu
	var psa PSAMenu
	menus := []MenuBase{
		rename.CreateMenu(0),
		pushWorkout.CreateMenu(0),
		deleteLastWorkout.CreateMenu(0),
		showUsers.CreateMenu(0),
		rejoinUser.CreateMenu(0),
		banUser.CreateMenu(0),
		groupLink.CreateMenu(0),
		manageAdmins.CreateMenu(0),
		removeUser.CreateMenu(0),
		updateRanks.CreateMenu(0),
		manageImmunity.CreateMenu(0),
		disputeWorkout.CreateMenu(0),
		psa.CreateMenu(0),
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
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}
