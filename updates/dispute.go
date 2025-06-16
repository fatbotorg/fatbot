package updates

import (
	"fatbot/users"
	"fmt"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handlePollUpdate(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	poll, err := users.GetWorkoutDisputePoll(update.PollAnswer.PollID)
	if err != nil {
		return fmt.Errorf("failed to get poll information: %v", err)
	}

	// Get group information to calculate required votes and get chat ID
	group, err := poll.GetGroup()
	if err != nil {
		return fmt.Errorf("failed to get group: %v", err)
	}

	// Get the current poll state from Telegram
	pollState, err := bot.StopPoll(tgbotapi.NewStopPoll(group.ChatID, poll.MessageID))
	if err != nil {
		return fmt.Errorf("failed to get poll state: %v", err)
	}
	log.Debug("pollstate %s", pollState.Question)

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
	}

	return nil
}
