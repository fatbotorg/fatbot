package updates

import (
	"fatbot/users"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (update CommandUpdate) handle() error {
	if err := handleCommandUpdate(update.FatBotUpdate); err != nil {
		return err
	}
	return nil
}

func (update UnknownGroupUpdate) handle() error {
	bot := update.Bot
	chatId := update.Update.FromChat().ID
	userId := update.Update.SentFrom().ID
	user, err := users.GetUserById(userId)
	if err != nil {
		return err
	}
	if user.IsAdmin &&
		update.Update.Message != nil &&
		update.Update.Message.IsCommand() &&
		update.Update.Message.Command() == "newgroup" {
		msg := handleNewGroupCommand(update.Update)
		if _, err := bot.Request(msg); err != nil {
			return err
		}
		return nil
	}
	bot.Send(tgbotapi.NewMessage(update.Update.Message.Chat.ID,
		fmt.Sprintf("Group %s not activated, send this to the admin: `%d`", update.Update.Message.Chat.Title, chatId),
	))
	sentry.CaptureMessage(fmt.Sprintf("non activated group: %d, title: %s", chatId, update.Update.FromChat().Title))
	return nil
}

func (update BlackListUpdate) handle() error {
	log.Debug("Blocked", "id", update.Update.SentFrom().ID)
	sentry.CaptureMessage(fmt.Sprintf("blacklist update: %d", update.Update.FromChat().ID))
	return nil
}

func (update CallbackUpdate) handle() error {
	if err := handleCallbacks(update.FatBotUpdate); err != nil {
		return err
	}
	return nil
}

func (update MediaUpdate) handle() error {
	if update.Update.FromChat().IsPrivate() {
		return nil
	}
	caption := update.Update.Message.Caption
	lines := strings.Split(strings.ToLower(caption), "\n")
	if caption != "" && strings.ReplaceAll(lines[0], " ", "") == "skip" {
		return nil
	}

	imageBytes, err := getFile(update)
	if err != nil {
		return err
	}
	labels := detectImageLabels(imageBytes)
	msg, err := handleWorkoutUpload(update, labels)
	if err != nil {
		return fmt.Errorf("Error handling last workout: %s", err)
	}
	if msg.Text == "" {
		return nil
	}
	if _, err := update.Bot.Send(msg); err != nil {
		return err
	}

	config := tgbotapi.SetMessageReactionConfig{
		ChatID:    update.Update.FromChat().ID,
		MessageID: update.Update.Message.MessageID,
		Reactions: []tgbotapi.ReactionType{{
			Type:  "emoji",
			Emoji: findReaction(labels),
		}},
	}
	if _, err := update.Bot.Request(config); err != nil {
		return err
	}
	return nil
}

func (update PrivateUpdate) handle() error {
	// Handle stateful callbacks first
	if err := handleStatefulCallback(update.FatBotUpdate); err == nil {
		return err
	}

	// Check if this is a weekly winner message reply
	if update.Update.Message != nil && update.Update.Message.ReplyToMessage != nil {
		// If it's a reply to a message from the bot
		if update.Update.Message.ReplyToMessage.From.ID == update.Bot.Self.ID {
			// And if the original message contains specific text about being the weekly winner
			if strings.Contains(update.Update.Message.ReplyToMessage.Text, "Congratulations on being this week's winner") {
				return handleWeeklyWinnerMessage(update.FatBotUpdate)
			}
		}
	}

	// Default response for private messages
	msg := tgbotapi.NewMessage(update.Update.FromChat().ID, "")
	msg.Text = "Try /help"
	if _, err := update.Bot.Send(msg); err != nil {
		return err
	}
	return nil
}

// handleWeeklyWinnerMessage processes the weekly winner's message and sends it to all their groups
func handleWeeklyWinnerMessage(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	bot := fatBotUpdate.Bot
	user, err := users.GetUserFromMessage(update.Message)
	if err != nil {
		return err
	}

	// Get the message text from the user
	message := update.Message.Text

	// Get all the groups the user is in
	if err := user.LoadGroups(); err != nil {
		log.Error("Failed to load user groups", "error", err)
		sentry.CaptureException(err)
		return err
	}

	// Check if the user has any groups
	if len(user.Groups) == 0 {
		log.Error("User has no groups", "user", user.GetName())
		sentry.CaptureException(fmt.Errorf("user has no groups: %s", user.GetName()))
		return nil
	}

	// Create a response to the user
	privateMsg := tgbotapi.NewMessage(
		user.TelegramUserID,
		"Thank you for your weekly message! It has been shared with your group(s).",
	)

	// Send the thank you message
	_, err = bot.Send(privateMsg)
	if err != nil {
		log.Error("Failed to send thank you message to winner", "error", err)
		sentry.CaptureException(err)
	}

	// For each group the user is in
	for _, group := range user.Groups {
		// Format the message to the group
		groupMsg := tgbotapi.NewMessage(
			group.ChatID,
			fmt.Sprintf("🏆 Weekly winner %s's message to the group:\n\n\"%s\"",
				user.GetName(),
				message),
		)

		// Send the message to the group
		sentMsg, err := bot.Send(groupMsg)
		if err != nil {
			log.Error("Failed to send weekly winner message to group", "error", err, "group", group.ChatID)
			sentry.CaptureException(err)
			continue
		}

		// Pin the message in the group
		pinChatMessageConfig := tgbotapi.PinChatMessageConfig{
			ChatID:              group.ChatID,
			MessageID:           sentMsg.MessageID,
			DisableNotification: false,
		}

		_, err = bot.Request(pinChatMessageConfig)
		if err != nil {
			log.Error("Failed to pin weekly winner message", "error", err)
			sentry.CaptureException(err)
		}

		// Register the weekly winner message event
		if err := user.RegisterWeeklyWinnerMessageEvent(message, group.ID); err != nil {
			log.Error("Failed to register weekly winner message event", "error", err)
			sentry.CaptureException(err)
		}
	}

	return nil
}
