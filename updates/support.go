package updates

import (
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

const (
	supportRedisPrefix       = "support:msg:"
	supportReverseMappingKey = "support:reply:"
	supportStateKey          = "support"
	supportStateTTL          = 300    // 5 minutes for awaiting user input
	supportMappingTTL        = 604800 // 7 days for message mapping
	supportCooldownTTL       = 60     // 60 seconds cooldown between support messages
	supportCooldownKey       = "support:cooldown:"
)

// getSupportGroupChatID returns the configured support group chat ID
func getSupportGroupChatID() int64 {
	return viper.GetInt64("support.group_chat_id")
}

// isSupportGroupConfigured checks if the support group is properly configured
func isSupportGroupConfigured() bool {
	return getSupportGroupChatID() != 0
}

// handleSupportCommand handles the /support command from a user in DM
func handleSupportCommand(fatBotUpdate FatBotUpdate) (tgbotapi.MessageConfig, error) {
	chatId := fatBotUpdate.Update.FromChat().ID
	msg := tgbotapi.NewMessage(chatId, "")

	if !isSupportGroupConfigured() {
		msg.Text = "Support is currently unavailable. Please try again later."
		log.Error("Support group not configured")
		return msg, nil
	}

	// Check cooldown
	cooldownKey := supportCooldownKey + fmt.Sprint(chatId)
	if _, err := state.Get(cooldownKey); err == nil {
		msg.Text = "Please wait before sending another support message."
		return msg, nil
	}

	// Set state to indicate we're waiting for a support message
	stateKey := fmt.Sprintf("%s:%d", supportStateKey, chatId)
	if err := state.SetWithTTL(stateKey, "awaiting", supportStateTTL); err != nil {
		log.Error("Failed to set support state", "error", err)
		return msg, err
	}

	msg.Text = "Please type your support message. You can ask for help or suggest a feature:"
	return msg, nil
}

// isSupportState checks if the user is currently in the support message flow
func isSupportState(chatId int64) bool {
	stateKey := fmt.Sprintf("%s:%d", supportStateKey, chatId)
	val, err := state.Get(stateKey)
	if err != nil {
		return false
	}
	return val == "awaiting"
}

// clearSupportState removes the support state for a user
func clearSupportState(chatId int64) {
	stateKey := fmt.Sprintf("%s:%d", supportStateKey, chatId)
	if err := state.ClearString(stateKey); err != nil {
		log.Error("Failed to clear support state", "error", err)
	}
}

// handleSupportMessage processes the user's support message and forwards it to the support group
func handleSupportMessage(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	chatId := update.FromChat().ID
	messageText := update.Message.Text

	if messageText == "" {
		clearSupportState(chatId)
		msg := tgbotapi.NewMessage(chatId, "Support messages currently only accept text. Please use /support and describe your issue in words.")
		if _, err := bot.Send(msg); err != nil {
			log.Error("Failed to send text-only notice", "error", err)
		}
		return nil
	}

	// Build user context for the support message
	supportMsg := buildSupportGroupMessage(update, messageText)

	// Send to support group
	supportGroupID := getSupportGroupChatID()
	log.Debug("Sending support message to group", "chatID", supportGroupID)
	groupMsg := tgbotapi.NewMessage(supportGroupID, supportMsg)
	groupMsg.ParseMode = "HTML"
	sentMsg, err := bot.Send(groupMsg)
	if err != nil {
		log.Error("Failed to send support message to group", "error", err)
		sentry.CaptureException(err)
		// Don't clear state so user can retry
		msg := tgbotapi.NewMessage(chatId, "Failed to send your support message. Please try again.")
		if _, sendErr := bot.Send(msg); sendErr != nil {
			log.Error("Failed to notify user of support send failure", "error", sendErr)
		}
		return err
	}

	// Clear state only after successful send to support group
	clearSupportState(chatId)

	// Store the mapping: support group message ID -> user's telegram ID
	if err := storeSupportMapping(sentMsg.MessageID, chatId); err != nil {
		log.Error("Failed to store support mapping", "error", err)
		sentry.CaptureException(err)
	}

	// Set cooldown
	cooldownKey := supportCooldownKey + fmt.Sprint(chatId)
	if err := state.SetWithTTL(cooldownKey, "1", supportCooldownTTL); err != nil {
		log.Error("Failed to set support cooldown", "error", err)
	}

	// Confirm to user
	confirmMsg := tgbotapi.NewMessage(chatId, "Your message has been sent to our support team. You'll receive a reply here.\n\nTo continue the conversation, reply directly to the support message you receive.")
	if _, err := bot.Send(confirmMsg); err != nil {
		log.Error("Failed to send support confirmation", "error", err)
		return err
	}

	return nil
}

// getSupportDisplayName returns the display name for a user in support messages.
// Uses the bot's GetName() (NickName or Name) for registered users,
// falls back to Telegram first/last name for unregistered users.
func getSupportDisplayName(from *tgbotapi.User) string {
	if user, err := users.GetUserById(from.ID); err == nil {
		return user.GetName()
	}
	displayName := from.FirstName
	if from.LastName != "" {
		displayName += " " + from.LastName
	}
	return displayName
}

// buildSupportGroupMessage creates the formatted message for the support group
func buildSupportGroupMessage(update tgbotapi.Update, messageText string) string {
	from := update.SentFrom()
	displayName := getSupportDisplayName(from)

	username := ""
	if from.UserName != "" {
		username = fmt.Sprintf(" (@%s)", from.UserName)
	}

	userContext := getUserContext(from.ID)

	return fmt.Sprintf(
		"<b>Support Request</b>\nFrom: %s%s\nUser ID: <code>%d</code>%s\n\n%s",
		displayName,
		username,
		from.ID,
		userContext,
		messageText,
	)
}

// buildSupportGroupFollowUp creates a formatted follow-up message with user context
func buildSupportGroupFollowUp(update tgbotapi.Update, messageText string) string {
	from := update.SentFrom()
	displayName := getSupportDisplayName(from)

	username := ""
	if from.UserName != "" {
		username = fmt.Sprintf(" (@%s)", from.UserName)
	}

	userContext := getUserContext(from.ID)

	return fmt.Sprintf(
		"<b>Follow-up</b>\nFrom: %s%s\nUser ID: <code>%d</code>%s\n\n%s",
		displayName,
		username,
		from.ID,
		userContext,
		messageText,
	)
}

// getUserContext returns additional user context from the database
func getUserContext(telegramUserID int64) string {
	user, err := users.GetUserById(telegramUserID)
	if err != nil {
		return "\nStatus: Unregistered"
	}

	var parts []string

	// Get groups
	if err := user.LoadGroups(); err == nil && len(user.Groups) > 0 {
		var groupNames []string
		for _, g := range user.Groups {
			groupNames = append(groupNames, g.Title)
		}
		parts = append(parts, fmt.Sprintf("Groups: %s", strings.Join(groupNames, ", ")))
	} else {
		parts = append(parts, "Groups: None")
	}

	// Active status
	if user.Active {
		parts = append(parts, "Status: Active")
	} else {
		parts = append(parts, "Status: Inactive")
	}

	// Rank
	ranks := users.GetRanks()
	if rank, ok := ranks[user.Rank]; ok {
		parts = append(parts, fmt.Sprintf("Rank: %s", rank.Name))
	}

	if len(parts) > 0 {
		return "\n" + strings.Join(parts, " | ")
	}
	return ""
}

// handleSupportGroupReply processes a reply in the support group and sends it back to the user
func handleSupportGroupReply(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot

	replyToMsgID := update.Message.ReplyToMessage.MessageID
	replyText := update.Message.Text

	if replyText == "" {
		// Only text replies are supported
		return nil
	}

	// Look up the original user from the mapping
	userTelegramID, err := lookupSupportMapping(replyToMsgID)
	if err != nil {
		log.Warn("Could not find support mapping for message", "messageID", replyToMsgID, "error", err)
		return nil
	}

	// Send reply to the user's DM
	userMsg := tgbotapi.NewMessage(userTelegramID, fmt.Sprintf("Support reply:\n\n%s", replyText))
	sentMsg, err := bot.Send(userMsg)
	if err != nil {
		log.Error("Failed to deliver support reply to user", "userID", userTelegramID, "error", err)
		sentry.CaptureException(err)

		// Notify support group about delivery failure
		failMsg := tgbotapi.NewMessage(
			getSupportGroupChatID(),
			fmt.Sprintf("Could not deliver reply to user %d — they may have blocked the bot.", userTelegramID),
		)
		failMsg.ReplyToMessageID = update.Message.MessageID
		if _, sendErr := bot.Send(failMsg); sendErr != nil {
			log.Error("Failed to send delivery failure notice to support group", "error", sendErr)
		}
		return nil
	}

	// Store reverse mapping: bot's DM message ID -> support group message ID
	// This allows the user to reply back and continue the conversation
	storeReverseMapping(sentMsg.MessageID, userTelegramID, replyToMsgID)

	return nil
}

// handleSupportFollowUp processes a user's reply to a support message in their DM
func handleSupportFollowUp(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	chatId := update.FromChat().ID
	replyToMsgID := update.Message.ReplyToMessage.MessageID
	messageText := update.Message.Text

	if messageText == "" {
		msg := tgbotapi.NewMessage(chatId, "Support messages currently only accept text.")
		if _, err := bot.Send(msg); err != nil {
			log.Error("Failed to send text-only notice", "error", err)
		}
		return nil
	}

	// Look up which support group message this is a reply to
	supportGroupMsgID, err := lookupReverseMapping(replyToMsgID, chatId)
	if err != nil {
		log.Warn("Could not find reverse support mapping", "messageID", replyToMsgID, "error", err)
		return nil
	}

	// Build follow-up message with full user context
	followUpMsg := tgbotapi.NewMessage(
		getSupportGroupChatID(),
		buildSupportGroupFollowUp(update, messageText),
	)
	followUpMsg.ParseMode = "HTML"
	followUpMsg.ReplyToMessageID = supportGroupMsgID

	sentMsg, err := bot.Send(followUpMsg)
	if err != nil {
		log.Error("Failed to send follow-up to support group", "error", err)
		sentry.CaptureException(err)
		msg := tgbotapi.NewMessage(chatId, "Failed to send your message. Please try again.")
		if _, sendErr := bot.Send(msg); sendErr != nil {
			log.Error("Failed to notify user of follow-up send failure", "error", sendErr)
		}
		return err
	}

	// Store mapping for the new message so support can reply to it too
	if err := storeSupportMapping(sentMsg.MessageID, chatId); err != nil {
		log.Error("Failed to store support mapping for follow-up", "error", err)
		sentry.CaptureException(err)
	}

	confirmMsg := tgbotapi.NewMessage(chatId, "Your follow-up has been sent to support.")
	if _, err := bot.Send(confirmMsg); err != nil {
		log.Error("Failed to send follow-up confirmation", "error", err)
	}

	return nil
}

// isSupportReply checks if a private message is a reply to a support message from the bot
func isSupportReply(update tgbotapi.Update) bool {
	if update.Message == nil || update.Message.ReplyToMessage == nil {
		return false
	}
	if !update.FromChat().IsPrivate() {
		return false
	}
	replyToMsgID := update.Message.ReplyToMessage.MessageID
	chatId := update.FromChat().ID
	_, err := lookupReverseMapping(replyToMsgID, chatId)
	return err == nil
}

// storeSupportMapping stores the mapping from support group message ID to user's telegram ID
func storeSupportMapping(supportGroupMsgID int, userTelegramID int64) error {
	key := supportRedisPrefix + fmt.Sprint(supportGroupMsgID)
	return state.SetWithTTL(key, fmt.Sprint(userTelegramID), supportMappingTTL)
}

// lookupSupportMapping retrieves the user's telegram ID from a support group message ID
func lookupSupportMapping(supportGroupMsgID int) (int64, error) {
	key := supportRedisPrefix + fmt.Sprint(supportGroupMsgID)
	val, err := state.Get(key)
	if err != nil {
		return 0, fmt.Errorf("no support mapping found for message %d: %w", supportGroupMsgID, err)
	}
	return strconv.ParseInt(val, 10, 64)
}

// storeReverseMapping stores a mapping from the bot's DM message ID back to the support group message ID
// Key includes the user's chat ID to avoid collisions between different users' message IDs
func storeReverseMapping(dmMsgID int, userChatID int64, supportGroupMsgID int) {
	key := fmt.Sprintf("%s%d:%d", supportReverseMappingKey, userChatID, dmMsgID)
	if err := state.SetWithTTL(key, fmt.Sprint(supportGroupMsgID), supportMappingTTL); err != nil {
		log.Error("Failed to store reverse support mapping", "error", err)
	}
}

// lookupReverseMapping retrieves the support group message ID from a bot DM message ID
func lookupReverseMapping(dmMsgID int, userChatID int64) (int, error) {
	key := fmt.Sprintf("%s%d:%d", supportReverseMappingKey, userChatID, dmMsgID)
	val, err := state.Get(key)
	if err != nil {
		return 0, fmt.Errorf("no reverse support mapping found for DM message %d: %w", dmMsgID, err)
	}
	msgID, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid reverse mapping value: %w", err)
	}
	return msgID, nil
}
