package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

//	func buildUser(update tgbotapi.Update) *User {
//		return &User{
//			Username:       update.Message.From.UserName,
//			Name:           update.Message.From.FirstName,
//			LastWorkout:    time.Now(),
//			ChatID:         update.Message.Chat.ID,
//			TelegramUserID: update.Message.From.ID,
//			IsActive:       true,
//		}
//	}
func main() {
	logger := log.New(os.Stderr)
	db()

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil {
		// panic(err)
		logger.Errorf(err.Error())
	}
	ticker := time.NewTicker(24 * time.Hour)
	done := make(chan bool)
	go func() {
		for {
			scanUsers(bot)
			select {
			case <-done:
				return
			case t := <-ticker.C:
				fmt.Println("Tick at", t)
			}
		}
	}()
	// time.Sleep(1600 * time.Millisecond)
	// ticker.Stop()
	// done <- true
	// fmt.Println("Ticker stopped")
	// scanUsers(bot)

	bot.Debug = false

	logger.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			// Respond to the callback query, telling Telegram to show the user
			// a message with the data received.
			// callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
			// if _, err := bot.Request(callback); err != nil {
			// 	panic(err)
			// }

			// recordUser := getUser(update.CallbackQuery.Message.Chat.UserName, update.CallbackQuery.Message.Chat.FirstName)
			// int64Data, err := strconv.ParseInt(update.CallbackQuery.Data, 10, 64)
			// if err != nil {
			// 	panic(err)
			// }
			// updateWorkout(update.CallbackQuery.Message.Chat.UserName, int64Data)
			//
			// var message string
			// if recordUser.LastWorkout.IsZero() {
			// 	log.Error("no last workout")
			// 	message = fmt.Sprintf("%s (%s) nice work!\nThis is your first workout",
			// 		update.Message.From.FirstName,
			// 		update.Message.From.UserName,
			// 	)
			// } else {
			// 	message = fmt.Sprintf("%s (%s) nice work!\nYour last workout was on %s (%d days ago)",
			// 		update.CallbackQuery.Message.From.FirstName,
			// 		update.CallbackQuery.Message.From.UserName,
			// 		recordUser.LastWorkout.Weekday(),
			// 		int(time.Now().Sub(recordUser.LastWorkout).Hours()/24),
			// 	)
			// }

			// And finally, send a message containing the data received.
			// msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, message)
			// if _, err := bot.Send(msg); err != nil {
			// 	panic(err)
			// }
		}

		if update.Message == nil { // ignore any non-Message updates
			continue
		}

		if !update.Message.IsCommand() { // ignore any non-command Messages
			if len(update.Message.Photo) > 0 {
				updateUserImage(update.Message.From.UserName)
			}
			continue
		}

		// Create a new MessageConfig. We don't have text yet,
		// so we leave it empty.
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		// Extract the command from the Message.
		switch update.Message.Command() {
		case "help":
			msg.Text = "I understand /sayhi and /status."
		case "status":
			user := getUser(update.Message)

			if user.LastWorkout.IsZero() {
				log.Warn("no last workout")
				msg.Text = "I don't have your last workout yet."
			} else {
				lastworkout := user.LastWorkout
				currentTime := time.Now()
				diff := currentTime.Sub(lastworkout)
				msg.Text = fmt.Sprintf("%s, your last workout was on %s\nYou have %d days left.",
					user.Name,
					user.LastWorkout.Weekday(),
					int(5-diff.Hours()/24))
			}
		case "show_users":
			users := getUsers()
			message := ""
			for _, user := range users {
				var lastWorkout string
				if user.LastWorkout.IsZero() {
					lastWorkout = "no record"
				} else {
					lastWorkout = user.LastWorkout.Weekday().String()
				}
				message = message + fmt.Sprintf("%s [%s]", user.Username, lastWorkout) + "\n"
			}
			msg.Text = message
		case "workout":
			// var daysKeyboard = tgbotapi.NewReplyKeyboard(
			// 	tgbotapi.NewKeyboardButtonRow(
			// 		tgbotapi.NewKeyboardButton("Today"),
			// 		tgbotapi.NewKeyboardButton("Yesterday"),
			// 		tgbotapi.NewKeyboardButton("Two days ago"),
			// 	),
			// )

			recordUser := getUser(update.Message)
			// int64Data, err := strconv.ParseInt(update..Data, 10, 64)
			if err != nil {
				panic(err)
			}
			var message string
			if recordUser.PhotoUpdate.IsZero() || time.Now().Sub(recordUser.PhotoUpdate).Hours() > 24 {
				message = fmt.Sprintf("%s, upload a photo and report again", recordUser.Username)
			} else {
				updateWorkout(update.Message.From.UserName, 0)
				if recordUser.LastWorkout.IsZero() {
					message = fmt.Sprintf("%s (%s) nice work!\nThis is your first workout",
						update.Message.From.FirstName,
						update.Message.From.UserName,
					)
				} else {
					hours := time.Now().Sub(recordUser.LastWorkout).Hours()
					// log.Info(hours)
					// log.Info(int64(hours))
					timeAgo := ""
					if int(hours/24) == 0 {
						timeAgo = fmt.Sprintf("%d hours ago", int(hours))
					} else {
						days := int(hours / 24)
						timeAgo = fmt.Sprintf("%d days and %d hours ago", days, int(hours)-days*24)
					}
					message = fmt.Sprintf("%s (%s) nice work!\nYour last workout was on %s (%s)",
						update.Message.From.FirstName,
						update.Message.From.UserName,
						recordUser.LastWorkout.Weekday(),
						timeAgo,
					)
				}
			}
			msg.Text = message

		case "admin_delete_last":
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
				super := member.IsAdministrator() || member.IsCreator()
				if super {
					// change last workout for user
					username := strings.Trim(update.Message.CommandArguments(), "@")
					err := rollbackLastWorkout(username)
					if err != nil {
						return
					}
					msg.Text = fmt.Sprintf("%s last workout cancelled", username)
				}
				// username := strings.Trim(update.Message.CommandArguments(), "@")
				// rollbackLastWorkout(username)
			}

			// user := getUser(update.Message.Chat.UserName, update.Message.Chat.FirstName)

		default:
			msg.Text = "Unknown"
		}

		if _, err := bot.Send(msg); err != nil {
			logger.Error(err)
		}
	}
}
