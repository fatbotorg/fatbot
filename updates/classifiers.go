package updates

import (
	"fatbot/users"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
)

func (fatBotUpdate FatBotUpdate) isCommandUpdate() bool {
	return fatBotUpdate.Update.Message != nil && fatBotUpdate.Update.Message.IsCommand()
}

func (fatBotUpdate FatBotUpdate) isMediaUpdate() bool {
	update := fatBotUpdate.Update
	return len(update.Message.Photo) > 0 || update.Message.Video != nil
}

func (fatBotUpdate FatBotUpdate) isPrivateUpdate() bool {
	return fatBotUpdate.Update.FromChat().IsPrivate()
}

func (fatBotUpdate FatBotUpdate) isCallbackUpdate() bool {
	if fatBotUpdate.Update.Message == nil {
		if fatBotUpdate.Update.CallbackQuery != nil {
			return true
		}
	} else {
		err := fmt.Errorf("cant read message, maybe I don't have access?")
		sentry.CaptureException(err)
		log.Error(err)
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
