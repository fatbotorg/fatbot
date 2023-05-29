package main

// func something() {
// TODO:
// Take everything below and apply to the above structure

// if !users.IsApprovedChatID(update.FromChat().ID) && !update.FromChat().IsPrivate() {
// 	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
// 		fmt.Sprintf("Group %s not activated, send this to the admin: `%d`", update.Message.Chat.Title, update.FromChat().ID),
// 	))
// 	sentry.CaptureMessage(fmt.Sprintf("non activated group: %d, title: %s", update.FromChat().ID, update.FromChat().Title))
// 	return nil
// }
// if users.BlackListed(update.SentFrom().ID) {
// 	log.Debug("Blocked", "id", update.SentFrom().ID)
// 	sentry.CaptureMessage(fmt.Sprintf("blacklist update: %d", update.FromChat().ID))
// 	return nil
// }
// if fatBotUpdate.Update.Message == nil {
// 	if fatBotUpdate.Update.CallbackQuery != nil {
// 		return handleCallbacks(fatBotUpdate)
// 	}
// 	if fatBotUpdate.Update.InlineQuery != nil {
// 		//NOTE: "Ignoring inline"
// 		return nil
// 	}
// 	return fmt.Errorf("Cant read message, maybe I don't have access?")
// }
// if !fatBotUpdate.Update.Message.IsCommand() {
// 	if err := handleNonCommandUpdates(fatBotUpdate); err != nil {
// 		return err
// 	}
// 	return nil
// }

// if err := handleCommandUpdate(fatBotUpdate); err != nil {
// 	return err
// }
// 	return nil
// }
