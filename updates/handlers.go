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

func (update PollUpdate) handle() error {
	log.Debug("poll !jupdate")
	return handlePollUpdate(update.Update, update.Bot)
}

func (update UnknownGroupUpdate) handle() error {
	bot := update.Bot
	if update.Update.FromChat() == nil {
		return nil
	}
	chatId := update.Update.FromChat().ID
	userId := update.Update.SentFrom().ID

	if update.Update.Message != nil &&
		update.Update.Message.IsCommand() &&
		update.Update.Message.Command() == "newgroup" {

		cmConfig := tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: chatId,
				UserID: userId,
			},
		}
		chatMember, err := bot.GetChatMember(cmConfig)
		if err != nil {
			return err
		}

		if chatMember.IsAdministrator() || chatMember.IsCreator() {
			msg := handleNewGroupCommand(update.Update)
			if _, err := bot.Request(msg); err != nil {
				return err
			}
			return nil
		}
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
		msg := update.Update.Message
		if msg.ReplyToMessage != nil && strings.Contains(msg.ReplyToMessage.Text, "Reply to this message with a photo") {
			chatId := update.Update.FromChat().ID
			user, err := users.GetUserById(chatId)
			if err != nil {
				return err
			}
			if err := user.LoadGroups(); err != nil {
				return err
			}

			count := 0
			for _, group := range user.Groups {
				// We can't forward a photo directly if we want to change caption or context, but Forward is simplest.
				// However, standard forwarding keeps the original sender.
				// To mimic the "Video Note" broadcasting behavior:
				forwardMsg := tgbotapi.NewForward(group.ChatID, chatId, msg.MessageID)
				if _, err := update.Bot.Send(forwardMsg); err != nil {
					log.Errorf("Failed to forward photo to group %d: %s", group.ChatID, err)
				} else {
					count++
				}
			}
			reply := tgbotapi.NewMessage(chatId, fmt.Sprintf("Sent photo to %d groups!", count))
			update.Bot.Send(reply)
			return nil
		}
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
	msg, err := handleWorkoutUpload(update, labels, imageBytes)
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

				user, err := users.GetUserFromMessage(update.Update.Message)
				if err != nil {
					return err
				}
				userName := user.GetName()
				replyMsg := tgbotapi.NewMessage(
					chatId,
					fmt.Sprintf("Thanks for your weekly message, %s! It has been pinned until next week's winner is announced.", userName),
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
		} else {
			log.Debug("Reply is not from the weekly leader",
				"sender_id", userId)
		}
	}
	return nil
}

func (update VideoNoteUpdate) handle() error {
	msg := update.Update.Message
	if msg.ReplyToMessage == nil {
		return nil
	}

	// Check if the reply is to the specific prompt
	if strings.Contains(msg.ReplyToMessage.Text, "Reply to this message with a video note") {
		chatId := update.Update.FromChat().ID
		
		// Broadcast to all user's groups
		user, err := users.GetUserById(chatId)
		if err != nil {
			return err
		}
		if err := user.LoadGroups(); err != nil {
			return err
		}

		count := 0
		for _, group := range user.Groups {
			forwardMsg := tgbotapi.NewForward(group.ChatID, chatId, msg.MessageID)
			if _, err := update.Bot.Send(forwardMsg); err != nil {
				log.Errorf("Failed to forward video note to group %d: %s", group.ChatID, err)
			} else {
				count++
			}
		}

		// Reply to user
		reply := tgbotapi.NewMessage(chatId, fmt.Sprintf("Sent to %d groups!", count))
		if _, err := update.Bot.Send(reply); err != nil {
			return err
		}
	}
	return nil
}

func (update FatBotUpdate) handle() error {
	if update.Update.Message != nil {
		if update.Update.Message.IsCommand() {
			if isAdminCommand(update.Update.Message.Command()) {
				return handleAdminCommandUpdate(update)
			}
			msg, err := handleCommand(update)
			if err != nil {
				return err
			}
			if msg.Text != "" {
				if _, err := update.Bot.Send(msg); err != nil {
					return err
				}
			}
		} else if update.Update.Message.Photo != nil {
			mediaUpdate := MediaUpdate{update}
			return mediaUpdate.handle()
		} else if update.Update.Message.VideoNote != nil {
			videoNoteUpdate := VideoNoteUpdate{update}
			return videoNoteUpdate.handle()
		} else if update.Update.FromChat().IsPrivate() {
			privateUpdate := PrivateUpdate{update}
			return privateUpdate.handle()
		}
	} else if update.Update.Poll != nil {
		log.Debug("GOT poll event : %s", update.Update.Poll)
		return handlePollUpdate(update.Update, update.Bot)
	}
	return nil
}

func handleCommand(update FatBotUpdate) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(update.Update.Message.Chat.ID, "")
	switch update.Update.Message.Command() {
	case "start":
		msg.Text = "Welcome to FatBot! Use /join to join a group."
	case "join":
		return handleJoinCommand(update)
	case "status":
		msg = handleStatusCommand(update.Update)
	case "stats":
		msg = handleStatsCommand(update.Update)
	case "help":
		msg.Text = "Available commands:\n/start - Start using the bot\n/join - Join a group\n/status - Check your workout status\n/stats - View workout statistics\n/help - Show this help message"
	default:
		msg.Text = "Unknown command. Use /help to see available commands."
	}
	return msg, nil
}
