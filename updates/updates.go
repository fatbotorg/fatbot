package updates

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type FatBotUpdate struct {
	Bot        *tgbotapi.BotAPI
	Update     tgbotapi.Update
	UpdateType UpdateType
}

type UpdateType interface {
	handle() error
}

type UnknownGroupUpdate struct {
	FatBotUpdate
}
type PrivateUpdate struct {
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
type VideoUpdate struct {
	FatBotUpdate
}
type PhotoUpdate struct {
	FatBotUpdate
}

func (fatBotUpdate *FatBotUpdate) classify() error {
	switch true {
	case fatBotUpdate.isUnknownGroupUpdate():
		fatBotUpdate.UpdateType = UnknownGroupUpdate{}
	case fatBotUpdate.isBlacklistUpdate():
		fatBotUpdate.UpdateType = BlackListUpdate{}
	case fatBotUpdate.isCallbackUpdate():
		fatBotUpdate.UpdateType = CallbackUpdate{}
	case fatBotUpdate.isCommandUpdate():
		fatBotUpdate.UpdateType = CommandUpdate{}
	default:
		err := fmt.Errorf("cannot classify update")
		sentry.CaptureException(err)
		return err
	}
	return nil
}
