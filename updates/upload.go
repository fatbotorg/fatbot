package updates

import (
	"fatbot/ai"
	"fatbot/users"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

func handleProbationUploadMessage(update tgbotapi.Update, user users.User) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(update.FromChat().ID, "")
	completedUploads, err := user.LastTwoWorkoutsInPastHour(update.FromChat().ID)
	if err != nil {
		return msg, nil
	}
	if completedUploads {
		msg.Text = fmt.Sprintf("%s, %s", user.GetName(), ai.GetAiWelcomeResponse())
		msg.ReplyToMessageID = update.Message.MessageID
	}
	return msg, nil
}

func getFile(update MediaUpdate) ([]byte, error) {
	var fileConfig tgbotapi.FileConfig
	numPhotos := len(update.Update.Message.Photo)
	if numPhotos > 0 {
		fileConfig.FileID = update.Update.Message.Photo[numPhotos-1].FileID
	} else if update.Update.Message.Video != nil {
		fileConfig.FileID = update.Update.Message.Video.Thumbnail.FileID
	}
	getFiles, err := update.Bot.GetFile(fileConfig)
	if err != nil {
		log.Error(err)
	}
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s",
		os.Getenv("TELEGRAM_APITOKEN"),
		getFiles.FilePath,
	)
	resp, err := http.Get(url)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func handleWorkoutUpload(update MediaUpdate) (tgbotapi.MessageConfig, error) {
	var message string
	botUpdate := update.Update
	msg := tgbotapi.NewMessage(botUpdate.Message.Chat.ID, "")
	user, err := users.GetUserById(botUpdate.SentFrom().ID)
	if err != nil {
		return msg, err
	}

	imageBytes, err := getFile(update)
	if err != nil {
		return msg, err
	}
	labels, points := detectImageLabels(imageBytes)
	chatId := botUpdate.FromChat().ID
	if !user.IsInGroup(chatId) {
		if err := user.RegisterInGroup(chatId); err != nil {
			log.Errorf("Error registering user %s in new group %d", user.GetName(), chatId)
			sentry.CaptureException(err)
		}
	}
	lastWorkout, err := user.GetLastXWorkout(1, botUpdate.FromChat().ID)
	if err != nil {
		log.Warn(err)
	}
	workOutOnceIn := viper.GetInt("workout.period")
	if !lastWorkout.IsOlderThan(workOutOnceIn) && !user.OnProbation {
		return msg, nil
	}
	var currentWorkout users.Workout
	if currentWorkout, err = user.UpdateWorkout(botUpdate, lastWorkout); err != nil {
		return msg, err
	}

	if user.OnProbation {
		return handleProbationUploadMessage(botUpdate, user)
	}

	if lastWorkout.CreatedAt.IsZero() {
		message = fmt.Sprintf("%s nice work!\nThis is your first workout",
			user.GetName(),
		)
	} else {
		hours := time.Now().Sub(lastWorkout.CreatedAt).Hours()
		timeAgo := ""
		if int(hours/24) == 0 {
			timeAgo = fmt.Sprintf("%d hours ago", int(hours))
		} else {
			days := int(hours / 24)
			timeAgo = fmt.Sprintf("%d days and %d hours ago", days, int(hours)-days*24)
		}
		if err := user.LoadWorkoutsThisCycle(chatId); err != nil {
			return msg, err
		}
		var streakMessage string
		if currentWorkout.Streak > 0 {
			streakSigns := ""
			for i := 0; i < currentWorkout.Streak; i++ {
				streakSigns += "👑"
			}
			streakMessage = fmt.Sprintf("%d in a row! %s %s", currentWorkout.Streak, streakSigns, users.GetRandomStreakMessage())
		}

		log.Debug(labels)
		message = fmt.Sprintf("%s %s\nLast workout: %s (%s)\nThis week: %d\n%s\n%s",
			user.GetName(),
			ai.GetAiResponse(labels),
			lastWorkout.CreatedAt.Weekday(),
			timeAgo,
			len(user.Workouts),
			streakMessage,
			fmt.Sprintf("Image quality: %d points!", points+1),
		)
	}

	msg.Text = message
	msg.ReplyToMessageID = botUpdate.Message.MessageID
	return msg, nil
}
