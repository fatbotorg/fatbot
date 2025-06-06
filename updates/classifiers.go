package updates

import (
	"fatbot/state"
	"fatbot/users"
	"strings"
)

func (fatBotUpdate FatBotUpdate) isCommandUpdate() bool {
	return fatBotUpdate.Update.Message != nil && fatBotUpdate.Update.Message.IsCommand()
}

func (fatBotUpdate FatBotUpdate) isMediaUpdate() bool {
	update := fatBotUpdate.Update
	return (update.Message != nil) && (len(update.Message.Photo) > 0 || update.Message.Video != nil)
}

func (fatBotUpdate FatBotUpdate) isPrivateUpdate() bool {
	return fatBotUpdate.Update.FromChat().IsPrivate()
}

func (fatBotUpdate FatBotUpdate) isCallbackUpdate() bool {
	if fatBotUpdate.Update.Message == nil {
		if fatBotUpdate.Update.CallbackQuery != nil {
			return true
		}
	}
	return false
}

func (fatBotUpdate FatBotUpdate) isUnknownGroupUpdate() bool {
	update := fatBotUpdate.Update

	if update.Poll != nil {
		chatID, err := state.PollMapping.GetPollChat(update.Poll.ID)
		if err != nil {
			return true
		}
		group, err := users.GetGroup(chatID)
		if err != nil {
			return true
		}
		return group.ChatID == 0
	}
	return !users.IsApprovedChatID(update.FromChat().ID) && !update.FromChat().IsPrivate()
}

// func (fatBotUpdate FatBotUpdate) isUnknownGroupUpdate() bool {
// 	update := fatBotUpdate.Update
// 	if update.Message != nil {
// 		if update.Message.Chat.Type == "private" {
// 			return false
// 		}
// 		group, err := users.GetGroup(update.Message.Chat.ID)
// 		if err != nil {
// 			return true
// 		}
// 		return group.ID == 0 // Check if group exists by checking if ID is set
// 	}
// 	if update.Poll != nil {
// 		chatID, err := state.PollMapping.GetPollChat(update.Poll.ID)
// 		if err != nil {
// 			return true
// 		}
// 		group, err := users.GetGroup(chatID)
// 		if err != nil {
// 			return true
// 		}
// 		return group.ID == 0 // Check if group exists by checking if ID is set
// 	}
// 	if update.FromChat() == nil {
// 		return false
// 	}
// 	// For other update types, check if they have a chat ID
// 	if update.PollAnswer != nil {
// 		// For poll answers, check if the poll belongs to a known group
// 		chatID, err := state.PollMapping.GetPollChat(update.PollAnswer.PollID)
// 		if err != nil {
// 			// If we can't get the chat ID from Redis, treat it as unknown
// 			return true
// 		}
// 		group, err := users.GetGroup(chatID)
// 		if err != nil {
// 			return true
// 		}
// 		return group.ID == 0 // Check if group exists by checking if ID is set
// 	}
// 	return true
// }

func (fatBotUpdate FatBotUpdate) isBlacklistUpdate() bool {
	return users.BlackListed(fatBotUpdate.Update.SentFrom().ID)
}

func isAdminCommand(cmd string) bool {
	commandPrefix := strings.Split(cmd, "_")
	if len(commandPrefix) > 0 && commandPrefix[0] == "admin" {
		return true
	}
	return false
}

func (fatBotUpdate FatBotUpdate) isGroupReplyUpdate() bool {
	update := fatBotUpdate.Update
	// Check if it's a message in a group chat (not private)
	if update.Message == nil || update.FromChat().IsPrivate() {
		return false
	}

	// Check if it's a reply to a bot message
	if update.Message.ReplyToMessage == nil || update.Message.ReplyToMessage.From == nil {
		return false
	}

	// Check if it's a reply to the bot
	if update.Message.ReplyToMessage.From.ID != fatBotUpdate.Bot.Self.ID {
		return false
	}

	// Check if it's a reply to the weekly message request
	if !strings.Contains(update.Message.ReplyToMessage.Text, "please share your weekly message as a reply to this message") {
		return false
	}

	return true
}

func (fatBotUpdate FatBotUpdate) isPollUpdate() bool {
	update := fatBotUpdate.Update
	return update.Poll != nil || update.PollAnswer != nil
}
