package updates

import (
	"fatbot/schedule"
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

// adminAttempts tracks unauthorized access attempts to admin commands
var (
	adminAttempts     = make(map[int64]int)
	adminAttemptMutex sync.Mutex
)

// Record an admin command attempt by a non-admin user
func recordAdminAttempt(userId int64) int {
	adminAttemptMutex.Lock()
	defer adminAttemptMutex.Unlock()

	// Record the attempt and return count
	adminAttempts[userId]++
	return adminAttempts[userId]
}

func handleCommandUpdate(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	if !update.FromChat().IsPrivate() {
		return nil
	}
	if isAdminCommand(update.Message.Command()) {
		return handleAdminCommandUpdate(fatBotUpdate)
	}
	var err error
	var msg tgbotapi.MessageConfig
	msg.Text = "Unknown command"
	switch update.Message.Command() {
	case "join", "start":
		msg, err = handleJoinCommand(fatBotUpdate)
		if err != nil {
			return err
		}
	case "status":
		msg = handleStatusCommand(update)
	case "stats":
		msg = handleStatsCommand(update)
	case "whoop":
		msg, err = HandleWhoopCommand(fatBotUpdate)
		if err != nil {
			return err
		}
	case "garmin":
		msg, err = HandleGarminCommand(fatBotUpdate)
		if err != nil {
			return err
		}
	case "help":
		msg.ChatID = update.FromChat().ID
		msg.Text = "Join the group using: /join\nCheck your status using: /status"
	default:
		msg.ChatID = update.FromChat().ID
	}
	if msg.Text == "" {
		return nil
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func handleNewGroupCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	groupChatId := update.Message.Chat.ID
	groupChatTitle := update.Message.Chat.Title
	msg := tgbotapi.NewMessage(groupChatId, "")
	switch update.FromChat().Type {
	case "private":
		return msg
	case "group":
		msg.Text = "not a supergroup, try creating an anonymous admin"
	case "supergroup":
		if err := users.CreateGroup(groupChatId, groupChatTitle); err != nil {
			log.Error("could not create group %s, id: %d", groupChatTitle, groupChatId)
			msg.Text = ""
		}
		msg.Text = fmt.Sprintf("group %s ready to start🏋️", groupChatTitle)
	}
	return msg
}

func handleStatsCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	var user users.User
	var err error
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	if user, err = users.GetUserFromMessage(update.Message); err != nil {
		log.Error(err)
		sentry.CaptureException(err)
	} else if user.ID == 0 {
		msg.Text = "Unregistered user"
		return msg
	}

	if chatIds, err := user.GetChatIds(); err != nil {
		log.Error(err)
		sentry.CaptureException(err)
		return msg
	} else {
		for _, chatId := range chatIds {
			group, _ := users.GetGroup(chatId)
			msg.Text = msg.Text +
				"\n\n" +
				fmt.Sprint(group.Title) +
				": " +
				schedule.CreateStatsMessage(chatId)
		}
	}
	return msg
}

func createRankStatusMessage(user *users.User) (string, error) {
	if user.RankUpdatedAt == nil {
		return "No workout history yet.", nil
	}
	ranks := users.GetRanks()
	currentRank := ranks[user.Rank]

	nextRank, ok := ranks[user.Rank+1]
	if !ok {
		return fmt.Sprintf("Current Rank: %s (highest rank!)",
			currentRank.Name), nil
	}

	daysSinceUpdate := int(time.Since(*user.RankUpdatedAt).Hours() / 24)
	daysNeededForNextRank := nextRank.MinDays - currentRank.MinDays
	remainingDays := daysNeededForNextRank - daysSinceUpdate

	return fmt.Sprintf(
		"Current Rank: %s\nDays until next rank (%s): %d",
		currentRank.Name,
		nextRank.Name,
		remainingDays,
	), nil
}

func handleStatusCommand(update tgbotapi.Update) tgbotapi.MessageConfig {
	var user users.User
	var err error
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

	if user, err = users.GetUserFromMessage(update.Message); err != nil {
		log.Error(err)
		sentry.CaptureException(err)
		msg.Text = "Failed to load user."
		return msg
	} else if user.ID == 0 {
		msg.Text = "Unregistered user."
		return msg
	}

	if chatIds, err := user.GetChatIds(); err != nil {
		log.Error(err)
		sentry.CaptureException(err)
		return msg
	} else {
		for _, chatId := range chatIds {
			group, _ := users.GetGroup(chatId)

			// Get rank status
			rankInfo, err := createRankStatusMessage(&user)
			if err != nil {
				rankInfo = ""
			}

			groupStatus := createStatusMessage(user, chatId, msg).Text

			msg.Text += "\n\n" +
				fmt.Sprintf("%s: %s\n%s", group.Title, rankInfo, groupStatus)
		}
	}

	return msg
}

func createStatusMessage(user users.User, chatId int64, msg tgbotapi.MessageConfig) tgbotapi.MessageConfig {
	lastWorkout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		log.Errorf("Err getting last workout: %s", err)
		sentry.CaptureException(err)
		return msg
	}
	if lastWorkout.CreatedAt.IsZero() {
		log.Warn("no last workout")
		msg.Text = "I don't have your last workout yet."
	} else {
		// Get the start of the day for both times to compare just the days
		isLastWorkoutOverdue, daysDiff := users.IsLastWorkoutOverdue(lastWorkout.CreatedAt)

		if isLastWorkoutOverdue {
			msg.Text = fmt.Sprintf("%s, your last workout was on %s\nYou are overdue for your workout!",
				user.GetName(),
				lastWorkout.CreatedAt.Weekday())
		} else {
			daysLeft := 5 - daysDiff
			msg.Text = fmt.Sprintf("%s, your last workout was on %s\nYou have %d days left to workout.",
				user.GetName(),
				lastWorkout.CreatedAt.Weekday(),
				daysLeft)
		}
	}
	return msg
}

func hasValidGroupCommandArgument(commandArguments string) (bool, users.Group) {
	joinArguments := strings.Split(commandArguments, " ")
	groupTitle := joinArguments[0]
	if group, err := users.GetGroupByTitle(groupTitle); err != nil {
		return false, users.Group{}
	} else {
		return true, group
	}
}

func getNameFromUpdate(update tgbotapi.Update) string {
	userName := update.SentFrom().UserName
	if userName != "" {
		return userName
	} else {
		return fmt.Sprintf("%s %s", update.SentFrom().FirstName, update.SentFrom().LastName)
	}
}

func sendLinkJoinForAdminApproval(fatBotUpdate FatBotUpdate, group users.Group) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	userId := fatBotUpdate.Update.FromChat().ID
	name := getNameFromUpdate(fatBotUpdate.Update)
	adminMessage := tgbotapi.NewMessage(0, fmt.Sprintf("%s wants to join using a link to %s, please approve", name, group.Title))
	approvalKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprintf("%d %d %s %s", group.ChatID, userId, name, fatBotUpdate.Update.SentFrom().UserName)),
			tgbotapi.NewInlineKeyboardButtonData("Block", fmt.Sprintf("%s %d", "block", userId)),
		),
	)
	adminMessage.ReplyMarkup = approvalKeyboard
	users.SendMessageToGroupAdmins(fatBotUpdate.Bot, group.ChatID, adminMessage)
	text := `Hello and welcome!
You will soon get a link to join the group 🎉.
Once you click the link, please send a picture of your workout *in the group chat*
‼️ NOTE ‼️: if you don't send a picture in the same day you get the link you will be BANNED from the group!`
	msg.Text = text
	return msg, nil
}

func handleJoinCommand(fatBotUpdate FatBotUpdate) (msg tgbotapi.MessageConfig, err error) {
	if user, err := users.GetUserById(fatBotUpdate.Update.SentFrom().ID); err != nil {
		if _, classificationErr := err.(*users.NoSuchUserError); classificationErr {
			return handleJoinCommandNewUser(fatBotUpdate)
		}
		return msg, err
	} else {
		return handleJoinCommandExistingUser(fatBotUpdate, user)
	}
}

func handleJoinCommandNewUser(fatBotUpdate FatBotUpdate) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	args := fatBotUpdate.Update.Message.CommandArguments()
	if hasValidGroup, group := hasValidGroupCommandArgument(args); hasValidGroup {
		return sendLinkJoinForAdminApproval(fatBotUpdate, group)
	}
	from := fatBotUpdate.Update.Message.From
	adminMessage := tgbotapi.NewMessage(0,
		fmt.Sprintf(
			"New join request: %s %s %s, choose a group",
			from.FirstName, from.LastName, from.UserName,
		),
	)
	adminMessage.ReplyMarkup = createNewUserGroupsKeyboard(from.ID, from.FirstName, from.UserName)
	users.SendMessageToSuperAdmins(fatBotUpdate.Bot, adminMessage)
	text := `Hello and welcome!
You will soon get a link to join the group 🎉.
Once you click the link, please send a picture of your workout *in the group chat*
‼️ NOTE ‼️: if you don't send a picture in the same day you get the link you will be BANNED from the group!`
	msg.Text = text
	return msg, nil
}

func handleJoinCommandExistingUser(fatBotUpdate FatBotUpdate, user users.User) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	if user.Active {
		msg.Text = "You are already active"
		return msg, nil
	}
	lastBanDate, err := user.GetLastBanDate()
	if err != nil {
		return msg, err
	}
	timeSinceBan := int(time.Now().Sub(lastBanDate).Hours())
	waitHours := viper.GetInt("ban.wait.hours")
	if timeSinceBan < waitHours {
		msg.Text = fmt.Sprintf("%s, it's only been %d hours, you have to wait %d", user.GetName(), timeSinceBan, waitHours)
	} else {
		if err := user.Rejoin(fatBotUpdate.Update, fatBotUpdate.Bot); err != nil {
			return msg, err
		}
	}
	return
}

func handleAdminCommandUpdate(fatBotUpdate FatBotUpdate) error {
	var msg tgbotapi.MessageConfig
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	msg.ChatID = update.FromChat().ID
	userId := update.Message.From.ID

	user, err := users.GetUserById(userId)
	if err != nil {
		return err
	}
	localAdmin, err := user.IsLocalAdmin()
	if err != nil {
		return err
	}

	// Check if the user is not an admin
	if !user.IsAdmin && !localAdmin {
		// Record the attempt and get the count
		attempts := recordAdminAttempt(userId)

		if attempts == 1 {
			// First attempt - send a warning
			msg.Text = "You are not authorized to use admin commands. Only group administrators can use this feature."
		} else if attempts < 5 {
			// Repeated attempts - send a stronger warning
			msg.Text = fmt.Sprintf("Warning: This is your %d attempt to access admin commands. Continued unauthorized attempts may result in being blocked.", attempts)
		} else {
			// Many attempts - send a final warning
			msg.Text = "FINAL WARNING: Your repeated attempts to access admin features have been logged. Further attempts will result in being blocked from using this bot."

			// Notify actual admins about the repeated attempts
			adminMsg := tgbotapi.NewMessage(0, fmt.Sprintf(
				"⚠️ Security Alert: User %s (ID: %d) has made %d attempts to access admin commands.",
				user.GetName(),
				userId,
				attempts,
			))
			users.SendMessageToSuperAdmins(bot, adminMsg)
		}

		if _, err := bot.Send(msg); err != nil {
			return err
		}
		return nil
	}

	switch update.Message.Command() {
	case "admin":
		msg = state.HandleAdminCommand(update)
	case "admin_send_report":
		schedule.CreateChart(bot)
	default:
		msg.Text = "Unknown command"
	}
	if msg.Text == "" {
		return nil
	}
	if _, err := bot.Send(msg); err != nil {
		return err
	}
	return nil
}
