package state

import (
	"fatbot/spotlight"
	"fatbot/users"
	"fmt"
	"strconv"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ActionData struct {
	Data   string
	Update tgbotapi.Update
	Bot    *tgbotapi.BotAPI
	State  *State
}

func (menu ManageAdminsMenu) PerformAction(params ActionData) error { return nil }

func (menu ShowAdminsMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}
	group, err := users.GetGroupWithAdmins(groupChatId)
	if err != nil {
		return nil
	}
	chatId := params.Update.FromChat().ID
	msg := tgbotapi.NewMessage(chatId, "")
	var adminsList string
	for _, admin := range group.Admins {
		adminsList += admin.GetName() + " "
	}
	msg.Text = adminsList
	if msg.Text == "" {
		msg.Text = fmt.Sprintf("No admins in group %s", group.Title)
	}
	if _, err := params.Bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (menu ChangeAdminsMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	option, err := params.State.getOption()
	if err != nil {
		return err
	}
	telegramUserId, err := params.State.getTelegramUserId()
	if err != nil {
		return err
	}
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}
	if user, err := users.GetUserById(telegramUserId); err != nil {
		return err
	} else {
		switch option {
		case "addadmin":
			if err := user.AddLocalAdmin(groupChatId); err != nil {
				return err
			}
		case "removeadmin":
			if err := user.RemoveLocalAdmin(groupChatId); err != nil {
				return err
			}
		default:
			log.Warn("Unknown", "option", option)
		}
	}
	return nil
}

func (menu GroupLinkMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}
	if group, err := users.GetGroup(groupChatId); err != nil {
		return err
	} else {
		chatId := params.Update.FromChat().ID
		msg := tgbotapi.NewMessage(chatId, "")
		msg.Text = fmt.Sprintf("https://t.me/%s?start=%s", params.Bot.Self.UserName, group.Title)
		if _, err := params.Bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func (menu BanUserMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	telegramUserId, err := params.State.getTelegramUserId()
	if err != nil {
		return err
	}
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}
	if user, err := users.GetUserById(telegramUserId); err != nil {
		return err
	} else {
		err := user.Ban(params.Bot, groupChatId)
		if err != nil {
			return err[0]
		}
	}
	return nil
}

func (menu RejoinUserMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	telegramUserId, err := params.State.getTelegramUserId()
	if err != nil {
		return err
	}
	if user, err := users.GetUserById(telegramUserId); err != nil {
		return err
	} else {
		err := user.Rejoin(params.Update, params.Bot)
		if err != nil {
			return err
		}
	}
	return nil
}

func (menu RenameMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	state := params.State
	telegramUserId, err := state.getTelegramUserId()
	if err != nil {
		return err
	}
	if user, err := users.GetUserById(telegramUserId); err != nil {
		return err
	} else {
		user.Rename(params.Data)
	}
	return nil
}

func (menu PushWorkoutMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	telegramUserId, err := params.State.getTelegramUserId()
	if err != nil {
		return err
	}
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}
	if user, err := users.GetUserById(telegramUserId); err != nil {
		return err
	} else {
		days, err := strconv.ParseInt(params.Data, 10, 64)
		if err != nil {
			return err
		}
		if err := user.PushWorkout(days, groupChatId); err != nil {
			return err
		}
	}
	return nil
}

func (menu DeleteLastWorkoutMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	groupChatId, _ := params.State.getGroupChatId()
	userId, _ := strconv.ParseInt(params.Data, 10, 64)
	user, err := users.GetUserById(userId)
	if err != nil {
		return err
	}
	if deletedWorkout, newLastWorkout, err := user.RollbackLastWorkout(groupChatId); err != nil {
		return err
	} else {
		if deletedWorkout.WhoopID != "" {
			SetWithTTL("whoop:ignored:"+deletedWorkout.WhoopID, "1", 604800) // 7 days
		}
		msg := tgbotapi.NewMessage(params.Update.FromChat().ID, "")
		message := fmt.Sprintf("Deleted last workout for user %s\nRolledback to: %s",
			user.GetName(), newLastWorkout.CreatedAt.Format("2006-01-02 15:04:05"))
		msg.Text = message
		if _, err := params.Bot.Send(msg); err != nil {
			return err
		}
		messageToUser := tgbotapi.NewMessage(0,
			fmt.Sprintf("Your last workout was cancelled by the admin.\nUpdated workout: %s",
				newLastWorkout.CreatedAt))
		user.SendPrivateMessage(params.Bot, messageToUser)
	}
	return nil
}

func (menu ShowUsersMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}
	users := users.GetGroupWithUsers(groupChatId).Users
	msg := tgbotapi.NewMessage(params.Update.FromChat().ID, "")
	message := ""
	var lastWorkoutStr string
	for _, user := range users {
		if !user.Active {
			continue
		}
		lastWorkout, err := user.GetLastXWorkout(1, groupChatId)
		if err != nil {
			log.Errorf("Err getting last workout: %s", err)
			continue
		}
		if lastWorkout.CreatedAt.IsZero() {
			lastWorkoutStr = "no record"
		} else {
			hour, min, _ := lastWorkout.CreatedAt.Clock()
			lastWorkoutStr = fmt.Sprintf("%s, %d:%d", lastWorkout.CreatedAt.Weekday().String(), hour, min)
		}
		message = message + fmt.Sprintf("%s [%s]", user.GetName(), lastWorkoutStr) + "\n"
	}
	msg.Text = message
	if msg.Text == "" {
		msg.Text = "No users to show"
	}
	if _, err := params.Bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (menu RemoveUserMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)

	// Get the confirmation response
	option, err := params.State.getOption()
	if err != nil {
		return err
	}

	// If user chose "no", don't proceed with deletion
	if option == "no" {
		msg := tgbotapi.NewMessage(params.Update.FromChat().ID, "User removal cancelled.")
		if _, err := params.Bot.Send(msg); err != nil {
			return err
		}
		return nil
	}

	// Only proceed if user confirmed with "yes"
	if option == "yes" {
		telegramUserId, err := params.State.getTelegramUserId()
		if err != nil {
			return err
		}

		groupChatId, err := params.State.getGroupChatId()
		if err != nil {
			return err
		}

		user, err := users.GetUserById(telegramUserId)
		if err != nil {
			return err
		}

		// First, ban the user from the group
		errs := user.Ban(params.Bot, groupChatId)
		if len(errs) > 0 {
			return errs[0]
		}

		// Now remove the user from the database
		if err := user.RemoveFromDatabase(); err != nil {
			return err
		}

		msg := tgbotapi.NewMessage(params.Update.FromChat().ID,
			fmt.Sprintf("User %s has been completely removed from the system.", user.GetName()))
		if _, err := params.Bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func (menu UpdateRanksMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	users.UpdateAllUserRanks()
	msg := tgbotapi.NewMessage(params.Update.FromChat().ID, "All user ranks have been updated.")
	params.Bot.Send(msg)
	return nil
}

func (menu ManageImmunityMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	telegramUserId, err := params.State.getTelegramUserId()
	if err != nil {
		return err
	}
	user, err := users.GetUserById(telegramUserId)
	if err != nil {
		return err
	}
	// Toggle immunity
	user.SetImmunity(true)
	msg := tgbotapi.NewMessage(params.Update.FromChat().ID, fmt.Sprintf("User %s immunity has been %s", user.GetName(), map[bool]string{true: "enabled", false: "disabled"}[user.Immuned]))
	params.Bot.Send(msg)
	return nil
}

func (menu DisputeWorkoutMenu) PerformAction(params ActionData) error {
	defer DeleteStateEntry(params.State.ChatId)
	telegramUserId, err := params.State.getTelegramUserId()
	if err != nil {
		return err
	}
	groupChatId, err := params.State.getGroupChatId()
	if err != nil {
		return err
	}

	user, err := users.GetUserById(telegramUserId)
	if err != nil {
		return err
	}

	// Get the user's last workout
	lastWorkout, err := user.GetLastXWorkout(1, groupChatId)
	if err != nil {
		msg := tgbotapi.NewMessage(params.Update.FromChat().ID, "No workout found for this user.")
		params.Bot.Send(msg)
		return nil
	}

	// Get the group to get its ID
	group, err := users.GetGroup(groupChatId)
	if err != nil {
		return err
	}

	// Create a poll in the group
	poll := tgbotapi.NewPoll(groupChatId, fmt.Sprintf("Cancel workout by %s from %s?", user.GetName(), lastWorkout.CreatedAt.Format("2006-01-02 15:04:05")))
	poll.Options = []string{"No", "Yes"}
	poll.IsAnonymous = false
	poll.Type = "regular"
	poll.Explanation = "Vote to decide if this workout should be cancelled"
	poll.OpenPeriod = 3600

	// Send the poll
	message, err := params.Bot.Send(poll)
	if err != nil {
		return err
	}

	// Store poll information
	if err := users.CreateWorkoutDisputePoll(
		message.Poll.ID,
		uint(group.ID),
		uint(user.ID),
		lastWorkout.ID,
		message.MessageID,
	); err != nil {
		return err
	}

	// Notify the target user
	userMsg := tgbotapi.NewMessage(telegramUserId,
		fmt.Sprintf("Your workout from %s is being disputed. A poll has been created in the group to decide the outcome.",
			lastWorkout.CreatedAt.Format("2006-01-02 15:04:05")))
	params.Bot.Send(userMsg)

	// Send confirmation to admin
	adminMsg := tgbotapi.NewMessage(params.Update.FromChat().ID,
		fmt.Sprintf("Dispute poll created for %s's workout. The poll will close in 1 hour or when enough votes are reached.", user.GetName()))
	params.Bot.Send(adminMsg)

	return nil
}

func (menu PSAMenu) PerformAction(params ActionData) error {
	chatId := params.Update.FromChat().ID
	option := params.Data

	switch option {
	case "cancel":
		defer DeleteStateEntry(chatId)
		ClearString(fmt.Sprintf("psa:stylized:%d", chatId))
		msg := tgbotapi.NewMessage(chatId, "PSA cancelled.")
		params.Bot.Send(msg)
		return nil

	case "edit":
		// Restart the flow by going back to the input step
		// Clear current stylized message
		ClearString(fmt.Sprintf("psa:stylized:%d", chatId))
		// We want to go back to the 'insertmessage' step
		// In the current menu system, we can restart the menu
		DeleteStateEntry(chatId)
		newMenu := PSAMenu{}.CreateMenu(chatId)
		msg := tgbotapi.NewMessage(chatId, "Please provide changes or a new message for the PSA:")
		params.Bot.Send(msg)
		// We re-create the state with just the menu name to trigger the first step
		CreateStateEntry(chatId, newMenu.Name)
		return &MenuActionDoneError{}

	case "approve":
		defer DeleteStateEntry(chatId)
		stylizedMessage, err := Get(fmt.Sprintf("psa:stylized:%d", chatId))
		if err != nil || stylizedMessage == "" {
			return fmt.Errorf("failed to retrieve stylized message: %s", err)
		}
		ClearString(fmt.Sprintf("psa:stylized:%d", chatId))

		groups := users.GetGroups()
		for _, group := range groups {
			if !group.Approved {
				continue
			}
			msg := tgbotapi.NewMessage(group.ChatID, stylizedMessage)
			msg.ParseMode = "Markdown"
			if _, err := params.Bot.Send(msg); err != nil {
				log.Errorf("failed to send PSA to group %s: %s", group.Title, err)
			}
		}
		confirmationMsg := tgbotapi.NewMessage(chatId, "PSA published to all groups.")
		params.Bot.Send(confirmationMsg)
		return nil

	default:
		msg := tgbotapi.NewMessage(chatId, "Please use the buttons to Approve, Edit, or Cancel the PSA.")
		params.Bot.Send(msg)
		return &MenuActionDoneError{}
	}
}

func (menu InstagramSpotlightMenu) PerformAction(params ActionData) error {
	data := params.Data
	chatId := params.Update.FromChat().ID
	adminUser, _ := users.GetUserById(chatId)

	if data == "random" {
		defer DeleteStateEntry(params.State.ChatId)
		// Immediately restore the admin menu
		if params.Update.CallbackQuery != nil {
			messageId := params.Update.CallbackQuery.Message.MessageID
			edit := tgbotapi.NewEditMessageTextAndMarkup(
				chatId, messageId, "Choose an option", CreateAdminKeyboard(adminUser.IsAdmin),
			)
			params.Bot.Request(edit)
		}
		// Notify the admin
		waitMsg := tgbotapi.NewMessage(chatId, "Random Instagram Spotlight triggered! Generating media and publishing, this may take up to a minute. Please wait...")
		params.Bot.Send(waitMsg)

		// Start the long process
		go spotlight.DailyInstagramAutomation(params.Bot)
		return &MenuActionDoneError{}
	} else if data == "pick" {
		return nil
	}

	steps := menu.CreateMenu(0).Steps
	lastStep := steps[len(steps)-1]
	if lastStep.Name == "chooseuserinsta" {
		defer DeleteStateEntry(params.State.ChatId)
		userId, err := strconv.ParseInt(data, 10, 64)
		if err != nil {
			return err
		}
		user, err := users.GetUserById(userId)
		if err != nil {
			return err
		}

		// Immediately restore the admin menu
		if params.Update.CallbackQuery != nil {
			messageId := params.Update.CallbackQuery.Message.MessageID
			edit := tgbotapi.NewEditMessageTextAndMarkup(
				chatId, messageId, "Choose an option", CreateAdminKeyboard(adminUser.IsAdmin),
			)
			params.Bot.Request(edit)
		}
		// Notify the admin
		waitMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf("Instagram Spotlight triggered for %s! Generating media and publishing, this may take up to a minute. Please wait...", user.GetName()))
		params.Bot.Send(waitMsg)

		// Start the long process
		go spotlight.ManualInstagramSpotlight(params.Bot, user)
		return &MenuActionDoneError{}
	}

	return nil
}
