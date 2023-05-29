package updates

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type FatBotUpdate struct {
	Bot    *tgbotapi.BotAPI
	Update tgbotapi.Update
}

type UpdateType interface {
	handle() error
}

type UnknownGroupUpdate struct {
	FatBotUpdate
}
type BlackListUpdate struct {
	FatBotUpdate
}
type CommandUpdate struct {
	FatBotUpdate
}
type CallbackUpdate struct {
	FatBotUpdate
}
type MediaUpdate struct {
	FatBotUpdate
}
type PrivateUpdate struct {
	FatBotUpdate
}

func (fatBotUpdate FatBotUpdate) classify() (UpdateType, error) {
	switch true {
	case fatBotUpdate.isUnknownGroupUpdate():
		return UnknownGroupUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isBlacklistUpdate():
		return BlackListUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isCallbackUpdate():
		return CallbackUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isCommandUpdate():
		return CommandUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isCommandUpdate():
		return CommandUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isMediaUpdate():
		return MediaUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isPrivateUpdate():
		return PrivateUpdate{FatBotUpdate: fatBotUpdate}, nil
	default:
		err := fmt.Errorf("cannot classify update")
		sentry.CaptureException(err)
		return nil, err
	}
}

func HandleUpdates(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	if update.SentFrom() == nil {
		err := fmt.Errorf("can't handle update with no SentFrom details...")
		sentry.CaptureException(err)
		return err
	}
	if updateType, err := fatBotUpdate.classify(); err != nil {
		return err
	} else {
		err := updateType.handle()
		if err != nil {
			return err
		}
	}
	return nil
}
