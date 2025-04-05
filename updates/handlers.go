package updates

import (
	"fatbot/users"
	"fmt"
	"strconv"
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

	// Default response for private messages
	msg := tgbotapi.NewMessage(update.Update.FromChat().ID, "")
	msg.Text = "Try /help"
	if _, err := update.Bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (update GroupReplyUpdate) handle() error {
	// Get the username or first name being addressed in the original message
	originalText := update.Update.Message.ReplyToMessage.Text
	// Extract the name from "🎤 [Name], as this week's first leader..."
	parts := strings.Split(originalText, ",")
	if len(parts) > 0 {
		// Extract the mentioned user from the markdown format: "[Name](tg://user?id=12345)"
		addressedText := strings.TrimPrefix(parts[0], "🎤 ")

		// Debug logging
		log.Debug("Processing weekly winner reply",
			"original_text", originalText,
			"addressed_text", addressedText,
			"sender_id", update.Update.Message.From.ID)

		// Try to extract the user ID from the tg://user?id= format if present
		var addressedUserID int64
		if strings.Contains(addressedText, "tg://user?id=") {
			idStart := strings.Index(addressedText, "tg://user?id=") + len("tg://user?id=")
			idEnd := strings.Index(addressedText[idStart:], ")")
			if idEnd > 0 {
				idStr := addressedText[idStart : idStart+idEnd]
				if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
					addressedUserID = id
					log.Debug("Extracted user ID from mention", "user_id", addressedUserID)
				}
			}
		}

		// Get the user from the database to check if they've already replied
		user, err := users.GetUserById(update.Update.Message.From.ID)
		if err != nil {
			log.Error("Failed to get user by ID", "error", err)
			sentry.CaptureException(err)
			return nil
		}

		// First check if the user IDs match (most reliable)
		isCorrectUser := addressedUserID > 0 && addressedUserID == update.Update.Message.From.ID

		// If we couldn't extract user ID or it doesn't match, fall back to name matching
		if !isCorrectUser {
			senderName := ""
			if update.Update.Message.From.UserName != "" {
				senderName = "@" + update.Update.Message.From.UserName
			} else {
				senderName = update.Update.Message.From.FirstName
			}

			// Clean the addressed name (remove markdown formatting)
			cleanAddressedName := addressedText
			if strings.Contains(addressedText, "[") && strings.Contains(addressedText, "]") {
				// Extract name from [Name](tg://user?id=12345)
				nameStart := strings.Index(addressedText, "[") + 1
				nameEnd := strings.Index(addressedText, "]")
				if nameEnd > nameStart {
					cleanAddressedName = addressedText[nameStart:nameEnd]
				}
			}

			log.Debug("Comparing names",
				"addressed_name", cleanAddressedName,
				"sender_name", senderName)

			// Check if names match
			isCorrectUser = strings.Contains(cleanAddressedName, senderName) ||
				strings.Contains(senderName, cleanAddressedName)
		}

		// Check if the person replying is the one being addressed and hasn't already replied
		if isCorrectUser && !user.HasRepliedToWeeklyMessage() {
			log.Info("Weekly winner replied to message, pinning their response")

			// Unpin all existing messages first
			unpinAllConfig := tgbotapi.UnpinAllChatMessagesConfig{
				ChatID: update.Update.Message.Chat.ID,
			}

			_, err := update.Bot.Request(unpinAllConfig)
			if err != nil {
				log.Error("Failed to unpin all messages", "error", err)
				sentry.CaptureException(err)
			}

			// Pin this message
			pinChatMessageConfig := tgbotapi.PinChatMessageConfig{
				ChatID:              update.Update.Message.Chat.ID,
				MessageID:           update.Update.Message.MessageID,
				DisableNotification: false,
			}

			_, err = update.Bot.Request(pinChatMessageConfig)
			if err != nil {
				log.Error("Failed to pin winner's message", "error", err)
				sentry.CaptureException(err)
			} else {
				// Register that the user has replied to the weekly message
				if err := user.RegisterWeeklyMessageRepliedEvent(); err != nil {
					log.Error("Failed to register weekly message reply event", "error", err)
					sentry.CaptureException(err)
				}

				// Extract name for thank you message
				thankName := update.Update.Message.From.FirstName
				if update.Update.Message.From.UserName != "" {
					thankName = "@" + update.Update.Message.From.UserName
				}

				// Thank the user for their message
				replyMsg := tgbotapi.NewMessage(
					update.Update.Message.Chat.ID,
					fmt.Sprintf("Thanks for your weekly message, %s! It has been pinned until next week's winner is announced.", thankName),
				)
				replyMsg.ReplyToMessageID = update.Update.Message.MessageID

				_, err = update.Bot.Send(replyMsg)
				if err != nil {
					log.Error("Failed to send thank you message", "error", err)
					sentry.CaptureException(err)
				}
			}
		} else if isCorrectUser && user.HasRepliedToWeeklyMessage() {
			// User has already replied, ignore this message
			log.Info("Weekly winner already replied once, ignoring additional replies",
				"user_id", user.ID,
				"name", user.GetName())

			// Optionally notify the user that they've already provided their weekly message
			replyMsg := tgbotapi.NewMessage(
				update.Update.Message.Chat.ID,
				fmt.Sprintf("You've already shared your weekly message, %s! Only your first reply is pinned.", user.GetName()),
			)
			replyMsg.ReplyToMessageID = update.Update.Message.MessageID

			_, err = update.Bot.Send(replyMsg)
			if err != nil {
				log.Error("Failed to send already replied message", "error", err)
				sentry.CaptureException(err)
			}
		} else {
			log.Debug("Reply is not from the weekly winner",
				"sender_id", update.Update.Message.From.ID,
				"addressed_user_id", addressedUserID)
		}
	}
	return nil
}
