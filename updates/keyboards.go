package updates

import (
	"fatbot/users"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func createNewUserGroupsKeyboard(userId int64, name, username string) tgbotapi.InlineKeyboardMarkup {
	groups := users.GetGroupsWithUsers()
	row := []tgbotapi.InlineKeyboardButton{}
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, group := range groups {
		groupLabel := fmt.Sprintf("%s", group.Title)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			groupLabel,
			fmt.Sprintf("%d %d %s %s", group.ChatID, userId, name, username),
		))
		if len(row) == 3 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 && len(row) < 3 {
		rows = append(rows, row)
	}
	blockRow := []tgbotapi.InlineKeyboardButton{}
	blockButton := tgbotapi.NewInlineKeyboardButtonData("Block", fmt.Sprintf("%s %d", "block", userId))
	blockRow = append(blockRow, blockButton)
	rows = append(rows, blockRow)

	var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
	return keyboard
}
