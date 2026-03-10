package updates

import (
	"fatbot/db"
	"fatbot/spotlight"
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"strings"
	"time"

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

	if update.Update.Message == nil {
		return nil
	}

	// Check if the bot is a member of this group at all.
	// If so, autonomous setup is in progress — stay silent and let
	// the MyChatMemberUpdate handler do its job.
	botMember, err := bot.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatId,
			UserID: bot.Self.ID,
		},
	})
	if err == nil && !botMember.HasLeft() && !botMember.WasKicked() {
		// Bot is in this group but group isn't registered yet.
		// If bot is admin in a supergroup, auto-register as fallback.
		if botMember.IsAdministrator() && update.Update.FromChat().Type == "supergroup" {
			if err := autoRegisterGroup(bot, update.Update); err != nil {
				log.Error("Failed to auto-register group", "err", err, "chat_id", chatId)
			}
		}
		// Either way, don't spam — setup is in progress.
		return nil
	}

	// Bot is NOT in this group — this is a genuinely unknown group.
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
		chatId := update.Update.FromChat().ID

		if msg.ReplyToMessage != nil && strings.Contains(msg.ReplyToMessage.Text, "Reply to this message with a photo") {
			user, err := users.GetUserById(chatId)
			if err != nil {
				return err
			}
			if err := user.LoadGroups(); err != nil {
				return err
			}

			fileId := msg.Photo[len(msg.Photo)-1].FileID
			count := 0
			for _, group := range user.Groups {
				// Update the latest workout in this group with the photo
				if lastWorkout, err := user.GetLastXWorkout(1, group.ChatID); err == nil {
					lastWorkout.PhotoFileID = fileId
					db.DBCon.Save(&lastWorkout)
				}

				forwardMsg := tgbotapi.NewForward(group.ChatID, chatId, msg.MessageID)
				if _, err := update.Bot.Send(forwardMsg); err != nil {
					log.Errorf("Failed to forward photo to group %d: %s", group.ChatID, err)
				} else {
					count++
				}
			}
			reply := tgbotapi.NewMessage(chatId, fmt.Sprintf("Sent photo to %d groups and saved for your daily progress! 📸", count))
			update.Bot.Send(reply)
			return nil
		}

		// Photo sent in private chat without a reply-to-prompt —
		// ask the user if they want to use it for their next workout upload.
		if len(msg.Photo) > 0 {
			fileId := msg.Photo[len(msg.Photo)-1].FileID
			promptMsg := tgbotapi.NewMessage(chatId,
				"Nice photo! Should I use this for your next workout when it gets uploaded through one of your integrations?")
			yesBtn := tgbotapi.NewInlineKeyboardButtonData("Yes, save it", fmt.Sprintf("photo:yes:%s", fileId))
			noBtn := tgbotapi.NewInlineKeyboardButtonData("No thanks", "photo:no")
			promptMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(yesBtn, noBtn),
			)
			// Pin the photo as a reply so context is clear
			promptMsg.ReplyToMessageID = msg.MessageID
			update.Bot.Send(promptMsg)
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

func (update SupportGroupReplyUpdate) handle() error {
	return handleSupportGroupReply(update.FatBotUpdate)
}

func (update PrivateUpdate) handle() error {
	// Check if user is in the support message flow
	chatId := update.Update.FromChat().ID
	if isSupportState(chatId) {
		return handleSupportMessage(update.FatBotUpdate)
	}

	// Check if user is replying to a support message (continued conversation)
	if isSupportReply(update.Update) {
		return handleSupportFollowUp(update.FatBotUpdate)
	}

	// Handle stateful callbacks first
	if err := handleStatefulCallback(update.FatBotUpdate); err == nil {
		return err
	}

	// Default response for private messages
	msg := tgbotapi.NewMessage(chatId, "")
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
		groupName := update.Update.Message.Chat.Title

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
			log.Infof("Weekly leader in group %s replied to message, pinning their response", groupName)

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

const (
	instaRateLimitTTL  = 2 * 24 * 60 * 60 // 2 days in seconds
	instaProcessingTTL = 120              // 2 minutes concurrency lock
)

func instaRateLimitKey(telegramUserID int64) string {
	return fmt.Sprintf("insta:user_request:%d", telegramUserID)
}

func instaProcessingKey(telegramUserID int64) string {
	return fmt.Sprintf("insta:processing:%d", telegramUserID)
}

// instaRemainingCooldown returns a human-readable string of how long until
// the user's rate limit expires, e.g. "1 day, 14 hours".
func instaRemainingCooldown(telegramUserID int64) string {
	key := instaRateLimitKey(telegramUserID)
	val, err := state.Get(key)
	if err != nil || val == "" {
		return ""
	}
	// val is the unix timestamp when the limit was set
	var setAt int64
	if _, err := fmt.Sscanf(val, "%d", &setAt); err != nil {
		return ""
	}
	expiresAt := time.Unix(setAt, 0).Add(time.Duration(instaRateLimitTTL) * time.Second)
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return ""
	}
	days := int(remaining.Hours()) / 24
	hours := int(remaining.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%d day(s), %d hour(s)", days, hours)
	}
	return fmt.Sprintf("%d hour(s)", hours)
}

func (update InstaRequestUpdate) handle() error {
	bot := update.Bot
	msg := update.Update.Message
	chatId := msg.Chat.ID
	senderID := msg.From.ID

	reply := func(text string) {
		r := tgbotapi.NewMessage(chatId, text)
		r.ReplyToMessageID = msg.MessageID
		bot.Send(r)
	}

	// 1. Look up the sender as a registered FatBot user
	user, err := users.GetUserById(senderID)
	if err != nil || !user.Active {
		log.Debugf("InstaRequest from unknown/inactive user %d", senderID)
		return nil // silently ignore
	}

	// 2. Must have an Instagram handle registered
	if user.InstagramHandle == "" {
		reply("Set your Instagram handle first with /instagram your_handle 📸")
		return nil
	}

	// 3. The replied-to message must be from the same user (own photo only)
	if msg.ReplyToMessage.From == nil || msg.ReplyToMessage.From.ID != senderID {
		reply("You can only request a spotlight for your own photos 🙅")
		return nil
	}

	// 4. Extract the FileID from the replied-to photo (largest size)
	replyPhotos := msg.ReplyToMessage.Photo
	if len(replyPhotos) == 0 {
		return nil // classifier already guards this, but be safe
	}
	photoFileID := replyPhotos[len(replyPhotos)-1].FileID

	// 5. Check 2-day rate limit
	if val, err := state.Get(instaRateLimitKey(senderID)); err == nil && val != "" {
		remaining := instaRemainingCooldown(senderID)
		if remaining != "" {
			reply(fmt.Sprintf("You've already been featured recently! You can request again in %s ⏳", remaining))
		} else {
			reply("You've already been featured recently! Try again in a bit ⏳")
		}
		return nil
	}

	// 6. Concurrency lock — prevent double-trigger during async processing
	if val, err := state.Get(instaProcessingKey(senderID)); err == nil && val != "" {
		reply("Your spotlight is already being processed, hang tight! ⚙️")
		return nil
	}
	if err := state.SetWithTTL(instaProcessingKey(senderID), "1", instaProcessingTTL); err != nil {
		log.Errorf("Failed to set insta processing lock: %s", err)
	}

	// 7. Parse optional custom caption (everything after "insta ")
	customCaption := ""
	text := strings.TrimSpace(msg.Text)
	if len(text) > 5 {
		customCaption = strings.TrimSpace(text[5:]) // strip "insta"
	}

	// 8. Acknowledge in the group
	reply("📸 Got it! Generating your Instagram spotlight... this may take a minute.")

	// 9. Set rate limit key BEFORE async work (timestamp value for TTL display)
	if err := state.SetWithTTL(
		instaRateLimitKey(senderID),
		fmt.Sprintf("%d", time.Now().Unix()),
		instaRateLimitTTL,
	); err != nil {
		log.Errorf("Failed to set insta rate limit key: %s", err)
	}

	// 10. Run the spotlight pipeline asynchronously
	go func() {
		defer state.ClearString(instaProcessingKey(senderID))
		spotlight.UserRequestedSpotlight(bot, user, photoFileID, chatId, customCaption)
	}()

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
