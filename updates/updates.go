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
type VideoNoteUpdate struct {
	FatBotUpdate
}
type PrivateUpdate struct {
	FatBotUpdate
}
type GroupReplyUpdate struct {
	FatBotUpdate
}
type PollUpdate struct {
	FatBotUpdate
}
type SupportGroupReplyUpdate struct {
	FatBotUpdate
}
type MyChatMemberUpdate struct {
	FatBotUpdate
}

func (fatBotUpdate FatBotUpdate) classify() (UpdateType, error) {
	switch {
	case fatBotUpdate.isMyChatMemberUpdate():
		return MyChatMemberUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isPollUpdate():
		return PollUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isBlacklistUpdate():
		return BlackListUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isUnknownGroupUpdate():
		return UnknownGroupUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isCallbackUpdate():
		return CallbackUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isCommandUpdate():
		return CommandUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isMediaUpdate():
		return MediaUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isVideoNoteUpdate():
		return VideoNoteUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isSupportGroupReplyUpdate():
		return SupportGroupReplyUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isGroupReplyUpdate():
		return GroupReplyUpdate{FatBotUpdate: fatBotUpdate}, nil
	case fatBotUpdate.isPrivateUpdate():
		return PrivateUpdate{FatBotUpdate: fatBotUpdate}, nil
	default:
		return nil, &NoSuchUpdateError{Update: fatBotUpdate.Update}
	}
}

func HandleUpdates(fatBotUpdate FatBotUpdate) error {
	update := fatBotUpdate.Update
	// MyChatMember updates have their own From field, not SentFrom
	if update.MyChatMember != nil {
		updateType := MyChatMemberUpdate{FatBotUpdate: fatBotUpdate}
		return updateType.handle()
	}
	if update.SentFrom() == nil && update.Poll != nil {
		return nil
	}
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
