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
		addressedName := strings.TrimPrefix(parts[0], "🎤 ")
		senderName := ""

		// Get the sender's name for comparison
		if update.Update.Message.From.UserName != "" {
			senderName = "@" + update.Update.Message.From.UserName
		} else {
			senderName = update.Update.Message.From.FirstName
		}

		// Check if the person replying is the one being addressed
		if strings.Contains(addressedName, senderName) || strings.Contains(senderName, addressedName) {
			// This is the winner replying with their weekly message
			// Pin this message
			pinChatMessageConfig := tgbotapi.PinChatMessageConfig{
				ChatID:              update.Update.Message.Chat.ID,
				MessageID:           update.Update.Message.MessageID,
				DisableNotification: false,
			}

			_, err := update.Bot.Request(pinChatMessageConfig)
			if err != nil {
				log.Error("Failed to pin winner's message", "error", err)
				sentry.CaptureException(err)
			} else {
				// Thank the user for their message
				replyMsg := tgbotapi.NewMessage(
					update.Update.Message.Chat.ID,
					fmt.Sprintf("Thanks for your weekly message, %s! It has been pinned until next week's winner is announced.", addressedName),
				)
				replyMsg.ReplyToMessageID = update.Update.Message.MessageID

				_, err = update.Bot.Send(replyMsg)
				if err != nil {
					log.Error("Failed to send thank you message", "error", err)
					sentry.CaptureException(err)
				}

				// Unpin the request message
				if update.Update.Message.ReplyToMessage != nil {
					_, err = update.Bot.Request(tgbotapi.UnpinChatMessageConfig{
						ChatID:    update.Update.Message.Chat.ID,
						MessageID: update.Update.Message.ReplyToMessage.MessageID,
					})
					if err != nil {
						log.Error("Failed to unpin request message", "error", err)
						sentry.CaptureException(err)
					}
				}
			}
		}
	}
	return nil
}
