package main

import (
	"fmt"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func isAdmin(userId int) bool {
	db := getDB()
	var user User
	db.Where("telegram_user_id = ?", userId).Find(&user)
	return user.IsAdmin
}

func handle_admin_delete_last_command(update tgbotapi.Update, bot *tgbotapi.BotAPI) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	config := &tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID:             update.FromChat().ID,
			SuperGroupUsername: "",
			UserID:             update.SentFrom().ID,
		},
	}
	member, err := bot.GetChatMember(*config)

	if err != nil {
		log.Error(err)
	} else {
		super := isAdmin(int(update.Message.From.ID)) && (member.IsAdministrator() || member.IsCreator())
		if super {

			users := getUsers()
			row := []tgbotapi.InlineKeyboardButton{}
			rows := [][]tgbotapi.InlineKeyboardButton{}
			for _, user := range users {
				userLabel := fmt.Sprintf("%s,%s", user.Name, user.Username)
				row = append(row, tgbotapi.NewInlineKeyboardButtonData(userLabel, fmt.Sprint(user.TelegramUserID)))
				if len(row) == 3 {
					rows = append(rows, row)
					row = row[:0]
				}
			}
			rows = append(rows, row)
			var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
			msg.ReplyMarkup = keyboard
			msg.Text = "Pick a user"
		}
	}
	return msg
}
