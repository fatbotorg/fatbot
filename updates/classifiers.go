package updates

import (
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
	return !users.IsApprovedChatID(update.FromChat().ID) && !update.FromChat().IsPrivate()
}

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
