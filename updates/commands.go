package updates

import (
	"fatbot/db"
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
	case "creategroup":
		msg, err = handleCreateGroupCommand(fatBotUpdate)
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
	case "strava":
		msg, err = HandleStravaCommand(fatBotUpdate)
		if err != nil {
			return err
		}
	case "instagram":
		err = handleInstagramCommand(fatBotUpdate)
		if err != nil {
			return err
		}
		return nil
	case "instagram_off":
		err = handleInstagramOffCommand(fatBotUpdate)
		if err != nil {
			return err
		}
		return nil
	case "support":
		msg, err = handleSupportCommand(fatBotUpdate)
		if err != nil {
			return err
		}
	case "help":
		msg.ChatID = update.FromChat().ID
		msg.Text = "Join a group: /join\nCreate your own group: /creategroup\nCheck your status: /status\nView stats: /stats"
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
	slug := joinArguments[0]
	// Try slug first (new flow), then fall back to title (legacy deep links)
	if group, err := users.GetGroupBySlug(slug); err == nil {
		return true, group
	}
	if group, err := users.GetGroupByTitle(slug); err == nil {
		return true, group
	}
	return false, users.Group{}
}

func handleCreateGroupCommand(fatBotUpdate FatBotUpdate) (msg tgbotapi.MessageConfig, err error) {
	msg.ChatID = fatBotUpdate.Update.FromChat().ID
	userId := fatBotUpdate.Update.SentFrom().ID

	// Feature flag check
	if !viper.GetBool("groups.creation.enabled") {
		msg.Text = "Group creation is currently disabled."
		return msg, nil
	}

	// Blacklist check
	if users.BlackListed(userId) {
		msg.Text = "You are blocked from using this bot."
		return msg, nil
	}

	// Check if user already has an autonomous group
	maxGroups := viper.GetInt64("groups.creation.max_per_user")
	if maxGroups == 0 {
		maxGroups = 1
	}
	if users.CountAutonomousGroupsByCreator(userId) >= maxGroups {
		msg.Text = "You already have a group. Each user can create one group."
		return msg, nil
	}

	botName := fatBotUpdate.Bot.Self.UserName
	msg.Text = fmt.Sprintf(`Let's create your group!

Follow these steps:

1. Open Telegram and create a new group
   Give it a short name (e.g. "Warriors")

2. Add @%s to the group

3. Make @%s an admin:
   Tap the group name > Edit > Administrators > Add @%s

4. Tap @%s > turn ON "Remain Anonymous" > then turn it back OFF

That's it! I'll set everything up automatically and send you an invite link to share with friends.`, botName, botName, botName, botName)

	// Notify super admins
	adminMsg := tgbotapi.NewMessage(0, fmt.Sprintf(
		"User %s (ID: %d) initiated group creation flow",
		getNameFromUpdate(fatBotUpdate.Update),
		userId,
	))
	users.SendMessageToSuperAdmins(fatBotUpdate.Bot, adminMsg)

	return msg, nil
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
	text := `Welcome!
You'll get a link to join the group soon.
Once you join, you have 5 days to post your first workout photo in the group chat.
After that, post at least once every 5 days to stay in!`
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
	text := `Welcome!
You'll get a link to join the group soon.
Once you join, you have 5 days to post your first workout photo in the group chat.
After that, post at least once every 5 days to stay in!`
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

func handleInstagramCommand(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	user, err := users.GetUserById(update.SentFrom().ID)
	if err != nil {
		return err
	}
	handle := strings.TrimSpace(update.Message.CommandArguments())
	if handle == "" {
		msg := tgbotapi.NewMessage(update.FromChat().ID, "Please provide your Instagram handle: `/instagram your_handle`")
		msg.ParseMode = "Markdown"
		_, err := bot.Send(msg)
		return err
	}
	// Remove @ if present
	handle = strings.TrimPrefix(handle, "@")
	user.InstagramHandle = handle
	if err := db.DBCon.Save(&user).Error; err != nil {
		return err
	}
	msg := tgbotapi.NewMessage(update.FromChat().ID, fmt.Sprintf("Awesome! I've registered your Instagram handle @%s and enabled daily automated stories. 🔥", handle))
	_, err = bot.Send(msg)
	return err
}

func handleInstagramOffCommand(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	user, err := users.GetUserById(update.SentFrom().ID)
	if err != nil {
		return err
	}
	user.InstagramHandle = ""
	if err := db.DBCon.Save(&user).Error; err != nil {
		return err
	}
	msg := tgbotapi.NewMessage(update.FromChat().ID, "Daily automated Instagram stories disabled. 🫡")
	_, err = bot.Send(msg)
	return err
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
