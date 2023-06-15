package updates

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type NoSuchUpdateError struct {
	Update tgbotapi.Update
}

func (e *NoSuchUpdateError) Error() string {
	return fmt.Sprintf("cannot classify update: %+v", e.Update)
}

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
	case fatBotUpdate.isMediaUpdate():
		return MediaUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isPrivateUpdate():
		return PrivateUpdate{FatBotUpdate: fatBotUpdate}, nil
	default:
		return nil, &NoSuchUpdateError{Update: fatBotUpdate.Update}
	}
}

func HandleUpdates(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	if update.SentFrom() == nil {
		return nil
	}
	if updateType, err := fatBotUpdate.classify(); err != nil {
		if _, classificationErr := err.(*NoSuchUpdateError); classificationErr {
			return nil
		}
		return err
	} else {
		err := updateType.handle()
		if err != nil {
			return err
		}
	}
	return nil
}
