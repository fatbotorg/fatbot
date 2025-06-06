package updates

import (
	"fatbot/state"
	"fatbot/users"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	DisputePollTimeout = 3600
)

func handleDisputeEvent(update tgbotapi.Update, bot *tgbotapi.BotAPI) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

	// Check if user is admin
	user, err := users.GetUserById(update.SentFrom().ID)
	if err != nil {
		return msg, fmt.Errorf("failed to get user: %v", err)
	}

	group, err := users.GetGroup(update.Message.Chat.ID)
	if err != nil {
		return msg, fmt.Errorf("failed to get group: %v", err)
	}

	if !user.IsGroupAdmin(group.ID) {
		msg.Text = "Only group admins can dispute workouts."
		return msg, nil
	}

	// Get target user from command arguments
	args := strings.Fields(update.Message.CommandArguments())
	if len(args) == 0 {
		msg.Text = "Please specify a user to dispute their last workout. Usage: /dispute @username"
		return msg, nil
	}

	// Extract user ID from mention or username
	var targetUserId int64
	if strings.HasPrefix(args[0], "@") {
		username := strings.TrimPrefix(args[0], "@")
		targetUser, err := users.GetUserByUsername(username)
		if err != nil {
			msg.Text = "Could not find user with that username."
			return msg, nil
		}
		targetUserId = targetUser.TelegramUserID
	} else {
		// Try to parse as user ID
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			msg.Text = "Please provide a valid username or user ID."
			return msg, nil
		}
		targetUserId = id
	}

	targetUser, err := users.GetUserById(targetUserId)
	if err != nil {
		msg.Text = "Could not find user."
		return msg, nil
	}

	// Get last workout
	lastWorkout, err := targetUser.GetLastXWorkout(1, update.Message.Chat.ID)
	if err != nil {
		msg.Text = fmt.Sprintf("Could not find last workout for %s.", targetUser.GetName())
		return msg, nil
	}

	// Create poll
	poll := tgbotapi.NewPoll(
		update.Message.Chat.ID,
		fmt.Sprintf("Should we count %s's workout from %s?",
			targetUser.GetName(),
			lastWorkout.CreatedAt.Format("2006-01-02 15:04:05")),
		"Yes, count the workout",
		"No, cancel the workout",
	)
	poll.IsAnonymous = true
	poll.Type = "quiz"
	poll.CorrectOptionID = 0 // Default to "Yes"
	poll.Explanation = "Vote to decide whether this workout should be counted."
	poll.OpenPeriod = DisputePollTimeout

	sentPoll, err := bot.Send(poll)
	if err != nil {
		log.Error("Failed to send poll", "error", err)
		sentry.CaptureException(err)
		msg.Text = "Failed to create dispute poll."
		return msg, err
	}

	// Store poll information in database
	if err := users.CreateWorkoutDisputePoll(
		sentPoll.Poll.ID,
		group.ID,
		targetUser.ID,
		lastWorkout.ID,
		sentPoll.MessageID,
	); err != nil {
		log.Error("Failed to store poll information", "error", err)
		sentry.CaptureException(err)
		msg.Text = "Failed to store poll information."
		return msg, err
	}

	// Store poll-to-chat mapping in Redis
	if err := state.PollMapping.StorePollChat(sentPoll.Poll.ID, update.Message.Chat.ID); err != nil {
		log.Error("Failed to store poll-to-chat mapping", "error", err)
		sentry.CaptureException(err)
		// Don't return error here as the poll is already created and stored in DB
	}

	msg.Text = fmt.Sprintf("Created dispute poll for %s's workout.", targetUser.GetName())
	return msg, nil
}

func handlePollUpdate(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	// Get poll information
	poll, err := users.GetWorkoutDisputePoll(update.Poll.ID)
	if err != nil {
		return fmt.Errorf("failed to get poll information: %v", err)
	}

	// Get chat ID from Redis
	chatID, err := state.PollMapping.GetPollChat(update.Poll.ID)
	if err != nil {
		return fmt.Errorf("failed to get chat ID from Redis: %v", err)
	}

	// Get the current poll state from Telegram
	pollState, err := bot.StopPoll(tgbotapi.NewStopPoll(chatID, poll.MessageID))
	if err != nil {
		return fmt.Errorf("failed to get poll state: %v", err)
	}
	log.Debug("pollstate %s", pollState.Question)

	// Get group information to calculate required votes
	group, err := poll.GetGroup()
	if err != nil {
		return fmt.Errorf("failed to get group: %v", err)
	}

	// Get active users in the group
	groupWithUsers := users.GetGroupWithUsers(group.ChatID)
	activeUsersCount := len(groupWithUsers.Users)

	// Calculate required votes: ([usersInGroup / 2] + 1)
	requiredVotes := (activeUsersCount / 2) + 1
	log.Debug("vote calculation", "active_users", activeUsersCount, "required_votes", requiredVotes)

	// Calculate votes for each option
	noVotes := pollState.Options[0].VoterCount
	yesVotes := pollState.Options[1].VoterCount

	// If we have enough votes for either option, process the decision
	if noVotes >= requiredVotes || yesVotes >= requiredVotes {
		// Get target user
		targetUser, err := poll.GetTargetUser()
		if err != nil {
			return fmt.Errorf("failed to get target user: %v", err)
		}

		if yesVotes >= requiredVotes {
			// Majority voted to cancel the workout
			workout, err := poll.GetWorkout()
			if err != nil {
				return fmt.Errorf("failed to get workout: %v", err)
			}

			_, err = targetUser.RollbackLastWorkout(group.ChatID)
			if err != nil {
				if _, nwe := err.(*users.NoWorkoutsError); !nwe {
					return err
				}
			}

			// Announce result
			msg := tgbotapi.NewMessage(group.ChatID,
				fmt.Sprintf("The group has decided to cancel %s's workout from %s.\nVotes: Yes: %d, No: %d (Required: %d)",
					targetUser.GetName(),
					workout.CreatedAt.Format("2006-01-02 15:04:05"),
					yesVotes,
					noVotes,
					requiredVotes))
			if _, err := bot.Send(msg); err != nil {
				return fmt.Errorf("failed to send group message: %v", err)
			}

			// Notify user
			userMsg := tgbotapi.NewMessage(targetUser.TelegramUserID,
				fmt.Sprintf("Your workout from %s was disputed and cancelled by group vote.",
					workout.CreatedAt.Format("2006-01-02 15:04:05")))
			if _, err := bot.Send(userMsg); err != nil {
				return fmt.Errorf("failed to send user message: %v", err)
			}
		} else {
			// Majority voted to keep the workout
			msg := tgbotapi.NewMessage(group.ChatID,
				fmt.Sprintf("The group has decided to keep %s's workout.\nVotes: Yes: %d, No: %d (Required: %d)",
					targetUser.GetName(),
					yesVotes,
					noVotes,
					requiredVotes))
			if _, err := bot.Send(msg); err != nil {
				return fmt.Errorf("failed to send group message: %v", err)
			}
		}

		// Clean up Redis entry after poll is processed
		if err := state.PollMapping.ClearPollChat(update.Poll.ID); err != nil {
			log.Error("Failed to clean up poll-to-chat mapping", "error", err)
			// Don't return error here as the poll is already processed
		}
	}

	return nil
}
