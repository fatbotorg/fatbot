package updates

import (
	"fatbot/users"
	"fmt"

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
	if strings.ToLower(update.Update.Message.Caption) == "skip" {
		return nil
	}
	msg, err := handleWorkoutUpload(update.Update)
	if err != nil {
		return fmt.Errorf("Error handling last workout: %s", err)
	}
	if msg.Text == "" {
		return nil
	}
	if _, err := update.Bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (update PrivateUpdate) handle() error {
	if err := handleStatefulCallback(update.FatBotUpdate); err == nil {
		return err
	}
	msg := tgbotapi.NewMessage(update.Update.FromChat().ID, "")
	msg.Text = "Try /help"
	if _, err := update.Bot.Send(msg); err != nil {
		return err
	}
	return nil
}
