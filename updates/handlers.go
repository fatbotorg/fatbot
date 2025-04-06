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
	// Get the original message text
	originalText := update.Update.Message.ReplyToMessage.Text

	// Check if this is a reply to the weekly leader message
	if strings.Contains(originalText, "as this week's first leader") {
		// Get the chat and user IDs
		chatId := update.Update.Message.Chat.ID
		userId := update.Update.Message.From.ID

		// Get the user from the database
		user, err := users.GetUserById(userId)
		if err != nil {
			log.Error("Failed to get user by ID", "error", err)
			sentry.CaptureException(err)
			return nil
		}

		// Check if this user is the weekly leader for this group and hasn't already replied
		isWeeklyLeader := user.IsWeeklyLeaderInGroup(chatId)
		hasReplied := user.HasRepliedToWeeklyMessage(chatId)

		log.Debug("Checking if user is weekly leader",
			"user_id", userId,
			"chat_id", chatId,
			"is_weekly_leader", isWeeklyLeader,
			"has_replied", hasReplied)

		if isWeeklyLeader && !hasReplied {
			log.Info("Weekly leader replied to message, pinning their response")

			// Unpin all existing messages first
			unpinAllConfig := tgbotapi.UnpinAllChatMessagesConfig{
				ChatID: chatId,
			}

			_, err := update.Bot.Request(unpinAllConfig)
			if err != nil {
				log.Error("Failed to unpin all messages", "error", err)
				sentry.CaptureException(err)
			}

			// Pin this message
			pinChatMessageConfig := tgbotapi.PinChatMessageConfig{
				ChatID:              chatId,
				MessageID:           update.Update.Message.MessageID,
				DisableNotification: false,
			}

			_, err = update.Bot.Request(pinChatMessageConfig)
			if err != nil {
				log.Error("Failed to pin winner's message", "error", err)
				sentry.CaptureException(err)
			} else {
				// Register that the user has replied to the weekly message
				if err := user.RegisterWeeklyMessageRepliedEvent(chatId); err != nil {
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
					chatId,
					fmt.Sprintf("Thanks for your weekly message, %s! It has been pinned until next week's winner is announced.", thankName),
				)
				replyMsg.ReplyToMessageID = update.Update.Message.MessageID

				_, err = update.Bot.Send(replyMsg)
				if err != nil {
					log.Error("Failed to send thank you message", "error", err)
					sentry.CaptureException(err)
				}
			}
		} else if isWeeklyLeader && hasReplied {
			// User has already replied, ignore this message
			log.Info("Weekly leader already replied once, ignoring additional replies",
				"user_id", user.ID,
				"name", user.GetName())

			// Optionally notify the user that they've already provided their weekly message
			replyMsg := tgbotapi.NewMessage(
				chatId,
				fmt.Sprintf("You've already shared your weekly message, %s! Only your first reply is pinned.", user.GetName()),
			)
			replyMsg.ReplyToMessageID = update.Update.Message.MessageID

			_, err = update.Bot.Send(replyMsg)
			if err != nil {
				log.Error("Failed to send already replied message", "error", err)
				sentry.CaptureException(err)
			}
		} else {
			log.Debug("Reply is not from the weekly leader",
				"sender_id", userId)
		}
	}
	return nil
}
