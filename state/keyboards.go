package state

import (
	"fatbot/users"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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

func CreateAdminKeyboard() tgbotapi.InlineKeyboardMarkup {
	var adminKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Rename User", "rename"),
			tgbotapi.NewInlineKeyboardButtonData("Push Workout", "pushworkout"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Delete Workout", "deletelastworkout"),
			tgbotapi.NewInlineKeyboardButtonData("Show Users", "showusers"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Show Events", "showevents"),
			tgbotapi.NewInlineKeyboardButtonData("Rejoin User", "rejoinuser"),
		),
	)
	return adminKeyboard
}
