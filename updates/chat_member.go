package updates

import (
	"fatbot/users"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

func (update MyChatMemberUpdate) handle() error {
	bot := update.Bot
	chatMember := update.Update.MyChatMember
	if chatMember == nil {
		return nil
	}

	oldStatus := chatMember.OldChatMember.Status
	newStatus := chatMember.NewChatMember.Status
	chatId := chatMember.Chat.ID
	chatTitle := chatMember.Chat.Title
	chatType := chatMember.Chat.Type
	from := chatMember.From

	log.Debug("MyChatMember update",
		"chat_id", chatId,
		"chat_title", chatTitle,
		"chat_type", chatType,
		"old_status", oldStatus,
		"new_status", newStatus,
		"from_id", from.ID,
	)

	// Bot was REMOVED from a group
	if newStatus == "left" || newStatus == "kicked" {
		return handleBotRemovedFromGroup(bot, chatMember)
	}

	// Bot was promoted to admin (could be from left→admin or member→admin)
	if newStatus == "administrator" {
		return handleBotPromotedToAdmin(bot, chatMember)
	}

	// Bot was added as regular member (not admin yet)
	if (oldStatus == "left" || oldStatus == "kicked") && newStatus == "member" {
		return handleBotAddedAsMember(bot, chatMember)
	}

	return nil
}

// handleBotAddedAsMember handles when the bot is added to a group but NOT as admin.
// We tell the user to make the bot admin — that's the only step they need.
// Once they promote the bot, handleBotPromotedToAdmin will fire and do the real setup.
func handleBotAddedAsMember(bot *tgbotapi.BotAPI, chatMember *tgbotapi.ChatMemberUpdated) error {
	if !viper.GetBool("groups.creation.enabled") {
		return nil
	}

	chatId := chatMember.Chat.ID
	chatType := chatMember.Chat.Type

	if chatType == "group" {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf(`Thanks for adding me!

I need to be an admin to work. Here's how:

1. Tap the group name > Edit > Administrators
2. Add @%s as admin
3. Turn ON "Remain Anonymous" for @%s
4. Turn it back OFF

This converts the group so I can manage it. I'll finish setup automatically!`, bot.Self.UserName, bot.Self.UserName))
		bot.Send(msg)
		return nil
	}

	if chatType == "supergroup" {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf(`Thanks for adding me!

One step left - make me an admin:
Tap the group name > Edit > Administrators > Add @%s

I'll finish setup automatically!`, bot.Self.UserName))
		bot.Send(msg)
		return nil
	}

	return nil
}

// handleBotPromotedToAdmin handles when the bot becomes an admin.
// This is the trigger for autonomous group setup.
func handleBotPromotedToAdmin(bot *tgbotapi.BotAPI, chatMember *tgbotapi.ChatMemberUpdated) error {
	if !viper.GetBool("groups.creation.enabled") {
		return nil
	}

	chatId := chatMember.Chat.ID
	chatType := chatMember.Chat.Type
	from := chatMember.From

	// Regular group with bot as admin — needs supergroup conversion.
	if chatType == "group" {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf(`Almost there!

1. Tap the group name > Edit > Administrators
2. Tap @%s > turn ON "Remain Anonymous"
3. Turn it back OFF

This converts the group so I can manage it. I'll finish setup automatically!`, bot.Self.UserName))
		bot.Send(msg)
		return nil
	}

	if chatType != "supergroup" {
		return nil
	}

	return setupAutonomousGroup(bot, chatId, chatMember.Chat.Title, &from)
}

// autoRegisterGroup is a fallback for when the bot is admin in a supergroup
// but the MyChatMemberUpdate was missed (e.g. bot was restarted during setup).
func autoRegisterGroup(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	return setupAutonomousGroup(bot, update.FromChat().ID, update.FromChat().Title, update.SentFrom())
}

// setupAutonomousGroup is the single source of truth for creating an autonomous group.
// Called by handleBotPromotedToAdmin (primary path) and autoRegisterGroup (fallback).
func setupAutonomousGroup(bot *tgbotapi.BotAPI, chatId int64, chatTitle string, from *tgbotapi.User) error {
	// Blacklist check
	if users.BlackListed(from.ID) {
		log.Warn("Blacklisted user tried to add bot to group", "user_id", from.ID, "chat_id", chatId)
		leaveConfig := tgbotapi.LeaveChatConfig{ChatID: chatId}
		bot.Request(leaveConfig)
		return nil
	}

	// Already registered — skip
	if users.GroupExistsByChatID(chatId) {
		log.Debug("Group already registered", "chat_id", chatId)
		return nil
	}

	// Group creation limit
	maxGroups := viper.GetInt64("groups.creation.max_per_user")
	if maxGroups == 0 {
		maxGroups = 1
	}
	if users.CountAutonomousGroupsByCreator(from.ID) >= maxGroups {
		msg := tgbotapi.NewMessage(chatId, "You already have a group. Each user can create one group.")
		bot.Send(msg)
		return nil
	}

	// Create the group
	group, err := users.CreateAutonomousGroup(chatId, chatTitle, from.ID)
	if err != nil {
		log.Error("Failed to create autonomous group", "err", err, "chat_id", chatId)
		sentry.CaptureException(err)
		return err
	}

	// Look up or create the user
	user, err := getOrCreateUser(from)
	if err != nil {
		return err
	}

	// Make creator a local admin
	if err := user.AddLocalAdmin(chatId); err != nil {
		log.Error("Failed to add local admin", "err", err)
		sentry.CaptureException(err)
	}

	// Register creator in the group
	if err := user.RegisterInGroup(chatId); err != nil {
		log.Error("Failed to register creator in group", "err", err)
		sentry.CaptureException(err)
	}

	// Build the invite link
	linkParam := group.Slug
	if linkParam == "" {
		linkParam = group.Title
	}
	inviteLink := fmt.Sprintf("https://t.me/%s?start=%s", bot.Self.UserName, linkParam)
	creatorName := user.GetName()

	// Send group activation message
	groupMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf(`Group activated!

How it works:
- Post a workout photo every 5 days
- Miss the deadline = banned (you can rejoin after 24h)
- Everyone starts with a 5-day grace period

%s is the group admin.
IMPORTANT❗: Do not add other users yourself, share the link below with them to register them to the group. 
%s`, creatorName, inviteLink))
	bot.Send(groupMsg)

	// Send private onboarding message to creator
	privateMsg := tgbotapi.NewMessage(from.ID, fmt.Sprintf(`You're the admin of "%s"!

Send this link to friends you want to invite:
%s

When they click it, you'll get a message to approve them.

Your admin tools (type /admin in our private chat):
- Group Link: get a fresh invite link anytime
- Show Users: see all members
- Ban User / Rejoin User: manage members
- Push Workout: credit a workout for someone
- Close Group: shut down the group

Need help? Type /admin anytime to see all options.`, chatTitle, inviteLink))
	bot.Send(privateMsg)

	// Notify super admins
	adminMsg := tgbotapi.NewMessage(0, fmt.Sprintf(
		"New autonomous group created: %s (ID: %d) by %s (ID: %d)",
		chatTitle, chatId, creatorName, from.ID,
	))
	users.SendMessageToSuperAdmins(bot, adminMsg)

	return nil
}

// getOrCreateUser looks up a user by Telegram ID, creating them if they don't exist.
func getOrCreateUser(from *tgbotapi.User) (users.User, error) {
	user, err := users.GetUserById(from.ID)
	if err != nil {
		if _, noSuchUser := err.(*users.NoSuchUserError); noSuchUser {
			name := from.FirstName
			if from.LastName != "" {
				name = name + " " + from.LastName
			}
			user = users.User{
				Username:       from.UserName,
				Name:           name,
				TelegramUserID: from.ID,
				Active:         true,
			}
			if err := user.CreateUser(); err != nil {
				log.Error("Failed to create user", "err", err)
				sentry.CaptureException(err)
				return user, err
			}
			user, err = users.GetUserById(from.ID)
			if err != nil {
				return user, err
			}
		} else {
			return user, err
		}
	}
	return user, nil
}

func handleBotRemovedFromGroup(bot *tgbotapi.BotAPI, chatMember *tgbotapi.ChatMemberUpdated) error {
	chatId := chatMember.Chat.ID
	chatTitle := chatMember.Chat.Title

	log.Info("Bot removed from group", "chat_id", chatId, "title", chatTitle)

	group, err := users.GetGroup(chatId)
	if err != nil || group.ID == 0 {
		return nil
	}

	creatorID := group.CreatorID

	// Deactivate all users who are only in this group
	if err := users.DeactivateGroupUsers(chatId); err != nil {
		log.Error("Failed to deactivate group users", "err", err)
		sentry.CaptureException(err)
	}

	// Mark group as not approved — stops all processing (strikes, reports, etc.)
	if err := users.UpdateGroupApproved(chatId, false); err != nil {
		log.Error("Failed to mark group as not approved", "err", err)
		sentry.CaptureException(err)
	}

	// Clear creator so their group slot is freed for a new group
	if err := users.ClearGroupCreator(chatId); err != nil {
		log.Error("Failed to clear group creator", "err", err)
		sentry.CaptureException(err)
	}

	// Notify the creator
	if creatorID != 0 {
		creatorMsg := tgbotapi.NewMessage(creatorID, fmt.Sprintf(
			"I was removed from \"%s\". The group has been deactivated.\n\nYou can create a new group anytime with /creategroup.",
			chatTitle,
		))
		bot.Send(creatorMsg)
	}

	// Notify super admins
	adminMsg := tgbotapi.NewMessage(0, fmt.Sprintf(
		"Bot removed from group \"%s\" (ID: %d). Group deactivated.",
		chatTitle, chatId,
	))
	users.SendMessageToSuperAdmins(bot, adminMsg)

	return nil
}
