package updates

import (
	"fatbot/users"
	"fatbot/whoop"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleWhoopCommand(fatBotUpdate FatBotUpdate) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(fatBotUpdate.Update.FromChat().ID, "")
	user, err := users.GetUserById(fatBotUpdate.Update.SentFrom().ID)
	if err != nil {
		return msg, err
	}

	state := fmt.Sprintf("fatbot-%d-%d", user.TelegramUserID, time.Now().Unix())
	// Force scope update by changing state prefix or just rely on the new scope constant in client
	authURL := whoop.GetAuthURL(state)
	log.Infof("Whoop Auth URL: %s", authURL)
	msg.Text = "Connect your Whoop account to automatically sync workouts."
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Connect Whoop", authURL),
		),
	)
	return msg, nil
}

func HandleWhoopCallback(w http.ResponseWriter, r *http.Request) {
	log.Infof("Whoop Callback received: %s", r.URL.String())
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	token, err := whoop.ExchangeToken(code)
	if err != nil {
		log.Errorf("Failed to exchange token: %s", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	parts := strings.Split(state, "-")
	if len(parts) < 2 {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}
	
	// parts[1] is the user ID based on "fatbot-USERID-TIMESTAMP"
	user, err := users.GetUserByState(parts[1])
	if err != nil {
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	if err := user.UpdateWhoopToken(token); err != nil {
		http.Error(w, "Failed to save token", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "Whoop connected successfully! You can close this window.")
}
