package updates

import (
	"fatbot/state"
	"fatbot/users"
	"fmt"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Track poll votes in memory (could be moved to database if needed)
var pollVotes = make(map[string]map[int64]int) // pollID -> userID -> optionIndex

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

	// Get active users in the group
	groupWithUsers := users.GetGroupWithUsers(group.ChatID)
	activeUsersCount := len(groupWithUsers.Users)

	// Calculate required votes: ([usersInGroup / 2] + 1)
	requiredVotes := (activeUsersCount / 2) + 1
	log.Debug("vote calculation", "active_users", activeUsersCount, "required_votes", requiredVotes)

	log.Debugf("%+v", update.PollAnswer)

	// Track the vote
	pollID := update.PollAnswer.PollID
	userID := update.PollAnswer.User.ID
	optionIndex := update.PollAnswer.OptionIDs[0] // Assuming single choice poll

	// Initialize vote tracking for this poll if not exists
	if pollVotes[pollID] == nil {
		pollVotes[pollID] = make(map[int64]int)
	}

	// Record the vote
	pollVotes[pollID][userID] = optionIndex

	// Count current votes
	noVotes := 0
	yesVotes := 0
	for _, vote := range pollVotes[pollID] {
		switch vote {
		case 0:
			noVotes++
		case 1:
			yesVotes++
		}
	}

	totalVotes := noVotes + yesVotes
	log.Debug("Current vote count", "no_votes", noVotes, "yes_votes", yesVotes, "total_votes", totalVotes, "active_users", activeUsersCount, "required", requiredVotes)

	// Check if there's a decision:
	// 1. One option has majority votes, OR
	// 2. Everyone has voted and it's a tie (default to NO)
	hasMajority := noVotes >= requiredVotes || yesVotes >= requiredVotes
	everyoneVoted := totalVotes >= activeUsersCount
	isTie := noVotes == yesVotes

	shouldResolve := hasMajority || (everyoneVoted && isTie)

	if shouldResolve {
		// Get target user
		targetUser, err := poll.GetTargetUser()
		if err != nil {
			return fmt.Errorf("failed to get target user: %v", err)
		}

		// Now stop the poll to get final confirmation
		pollState, err := bot.StopPoll(tgbotapi.NewStopPoll(group.ChatID, poll.MessageID))
		if err != nil {
			return fmt.Errorf("failed to stop poll: %v", err)
		}
		log.Debug("Final poll state", "question", pollState.Question)

		// Use the final poll state for confirmation
		finalNoVotes := pollState.Options[0].VoterCount
		finalYesVotes := pollState.Options[1].VoterCount

		// Determine the outcome
		// If it's a tie and everyone voted, default to NO (keep workout)
		// Otherwise, use majority rule
		keepWorkout := false
		if everyoneVoted && finalNoVotes == finalYesVotes {
			// Tie with everyone voting - default to NO (keep workout)
			keepWorkout = true
			log.Debug("Tie with everyone voting - defaulting to NO (keep workout)")
		} else if finalYesVotes >= requiredVotes {
			// Majority voted to cancel the workout
			keepWorkout = false
		} else {
			// Majority voted to keep the workout (or no majority reached)
			keepWorkout = true
		}

		if !keepWorkout {
			// Cancel the workout
			workout, err := poll.GetWorkout()
			if err != nil {
				return fmt.Errorf("failed to get workout: %v", err)
			}

			deletedWorkout, _, err := targetUser.RollbackLastWorkout(group.ChatID)
			if err != nil {
				if _, nwe := err.(*users.NoWorkoutsError); !nwe {
					return err
				}
			}

			if deletedWorkout.WhoopID != "" {
				state.SetWithTTL("whoop:ignored:"+deletedWorkout.WhoopID, "1", 604800) // 7 days
			}

			// Announce result
			msg := tgbotapi.NewMessage(group.ChatID,
				fmt.Sprintf("The group has decided to cancel %s's workout from %s.\nVotes: Yes: %d, No: %d (Required: %d)",
					targetUser.GetName(),
					workout.CreatedAt.Format("2006-01-02 15:04:05"),
					finalYesVotes,
					finalNoVotes,
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
			// Keep the workout
			msg := tgbotapi.NewMessage(group.ChatID,
				fmt.Sprintf("The group has decided to keep %s's workout.\nVotes: Yes: %d, No: %d (Required: %d)",
					targetUser.GetName(),
					finalYesVotes,
					finalNoVotes,
					requiredVotes))
			if _, err := bot.Send(msg); err != nil {
				return fmt.Errorf("failed to send group message: %v", err)
			}
		}

		// Clean up vote tracking for this poll
		delete(pollVotes, pollID)
	} else {
		// No decision yet - let the poll continue
		log.Debug("No decision yet", "no_votes", noVotes, "yes_votes", yesVotes, "total_votes", totalVotes, "active_users", activeUsersCount, "required", requiredVotes)
	}

	return nil
}
