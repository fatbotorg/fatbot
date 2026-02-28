package updates

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"fatbot/db"
	"fatbot/users"
)

// setupTestDB initializes an in-memory SQLite database for testing.
// It sets db.DBCon and runs AutoMigrate for the user-related models.
func setupTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		NowFunc: func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	db.DBCon = testDB
	if err := testDB.AutoMigrate(
		&users.User{},
		&users.Group{},
		&users.Workout{},
	); err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}
}

// makeUpdateWithUser builds a tgbotapi.Update with a Message.From user for testing.
func makeUpdateWithUser(userID int64, firstName, lastName, userName string) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{
				ID:        userID,
				FirstName: firstName,
				LastName:  lastName,
				UserName:  userName,
			},
			Chat: &tgbotapi.Chat{
				ID:   userID,
				Type: "private",
			},
		},
	}
}

func TestSupportBuildSupportGroupMessage(t *testing.T) {
	setupTestDB(t)

	var tests = []struct {
		name        string
		userID      int64
		firstName   string
		lastName    string
		userName    string
		messageText string
		wantHeader  string
		wantUser    string
		wantID      string
		wantBody    string
		wantNoAt    bool // true if we expect NO (@...) in the output
	}{
		{
			name:        "full user with username",
			userID:      12345,
			firstName:   "John",
			lastName:    "Doe",
			userName:    "johndoe",
			messageText: "I need help with workouts",
			wantHeader:  "<b>Support Request</b>",
			wantUser:    "From: John Doe (@johndoe)",
			wantID:      "User ID: <code>12345</code>",
			wantBody:    "I need help with workouts",
			wantNoAt:    false,
		},
		{
			name:        "user without username",
			userID:      67890,
			firstName:   "Jane",
			lastName:    "Smith",
			userName:    "",
			messageText: "Feature request: dark mode",
			wantHeader:  "<b>Support Request</b>",
			wantUser:    "From: Jane Smith",
			wantID:      "User ID: <code>67890</code>",
			wantBody:    "Feature request: dark mode",
			wantNoAt:    true,
		},
		{
			name:        "user without last name",
			userID:      11111,
			firstName:   "Alice",
			lastName:    "",
			userName:    "alice_bot",
			messageText: "Bug report",
			wantHeader:  "<b>Support Request</b>",
			wantUser:    "From: Alice (@alice_bot)",
			wantID:      "User ID: <code>11111</code>",
			wantBody:    "Bug report",
			wantNoAt:    false,
		},
		{
			name:        "user with first name only",
			userID:      99999,
			firstName:   "Bob",
			lastName:    "",
			userName:    "",
			messageText: "Hello support",
			wantHeader:  "<b>Support Request</b>",
			wantUser:    "From: Bob",
			wantID:      "User ID: <code>99999</code>",
			wantBody:    "Hello support",
			wantNoAt:    true,
		},
		{
			name:        "empty message text",
			userID:      55555,
			firstName:   "Eve",
			lastName:    "Test",
			userName:    "evetest",
			messageText: "",
			wantHeader:  "<b>Support Request</b>",
			wantUser:    "From: Eve Test (@evetest)",
			wantID:      "User ID: <code>55555</code>",
			wantBody:    "",
			wantNoAt:    false,
		},
		{
			name:        "message with special characters",
			userID:      77777,
			firstName:   "Test",
			lastName:    "User",
			userName:    "testuser",
			messageText: "Help! My <b>workout</b> didn't save & I'm frustrated",
			wantHeader:  "<b>Support Request</b>",
			wantUser:    "From: Test User (@testuser)",
			wantID:      "User ID: <code>77777</code>",
			wantBody:    "Help! My <b>workout</b> didn't save & I'm frustrated",
			wantNoAt:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := makeUpdateWithUser(tt.userID, tt.firstName, tt.lastName, tt.userName)
			result := buildSupportGroupMessage(update, tt.messageText)

			if !strings.Contains(result, tt.wantHeader) {
				t.Errorf("missing header: want %q in result:\n%s", tt.wantHeader, result)
			}
			if !strings.Contains(result, tt.wantUser) {
				t.Errorf("missing user line: want %q in result:\n%s", tt.wantUser, result)
			}
			if !strings.Contains(result, tt.wantID) {
				t.Errorf("missing user ID: want %q in result:\n%s", tt.wantID, result)
			}
			if !strings.Contains(result, tt.wantBody) {
				t.Errorf("missing body: want %q in result:\n%s", tt.wantBody, result)
			}
			if tt.wantNoAt && strings.Contains(result, "(@") {
				t.Errorf("expected no username mention but found (@) in result:\n%s", result)
			}
		})
	}
}

func TestSupportBuildSupportGroupMessageFormat(t *testing.T) {
	setupTestDB(t)

	update := makeUpdateWithUser(42, "Omer", "Hamerman", "omerxx")
	result := buildSupportGroupMessage(update, "Test message")

	// The user doesn't exist in the test DB, so getUserContext returns "\nStatus: Unregistered"
	lines := strings.Split(result, "\n")

	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d: %q", len(lines), result)
	}

	if lines[0] != "<b>Support Request</b>" {
		t.Errorf("line 0: got %q, want %q", lines[0], "<b>Support Request</b>")
	}
	if lines[1] != "From: Omer Hamerman (@omerxx)" {
		t.Errorf("line 1: got %q, want %q", lines[1], "From: Omer Hamerman (@omerxx)")
	}
	if !strings.HasPrefix(lines[2], "User ID: <code>42</code>") {
		t.Errorf("line 2: got %q, want prefix %q", lines[2], "User ID: <code>42</code>")
	}
}

func TestSupportDisplayNameConstruction(t *testing.T) {
	setupTestDB(t)

	var tests = []struct {
		name      string
		firstName string
		lastName  string
		wantName  string
	}{
		{"both names", "John", "Doe", "From: John Doe"},
		{"first only", "John", "", "From: John"},
		{"unicode names", "Алексей", "Иванов", "From: Алексей Иванов"},
		{"emoji in name", "🎉Party", "Guy", "From: 🎉Party Guy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := makeUpdateWithUser(1, tt.firstName, tt.lastName, "")
			result := buildSupportGroupMessage(update, "test")
			if !strings.Contains(result, tt.wantName) {
				t.Errorf("got result:\n%s\nwant to contain: %q", result, tt.wantName)
			}
		})
	}
}

func TestSupportUsernameFormatting(t *testing.T) {
	setupTestDB(t)

	// Verify username is wrapped in (@...) format when present
	update := makeUpdateWithUser(1, "Test", "", "myuser")
	result := buildSupportGroupMessage(update, "msg")
	if !strings.Contains(result, "(@myuser)") {
		t.Errorf("expected (@myuser) in result, got:\n%s", result)
	}

	// Verify no empty parens when username is absent
	update2 := makeUpdateWithUser(2, "Test", "", "")
	result2 := buildSupportGroupMessage(update2, "msg")
	if strings.Contains(result2, "(@)") || strings.Contains(result2, "( )") || strings.Contains(result2, "()") {
		t.Errorf("expected no empty parens in result, got:\n%s", result2)
	}
}

func TestSupportGetUserContext(t *testing.T) {
	setupTestDB(t)

	t.Run("unregistered user returns Unregistered status", func(t *testing.T) {
		result := getUserContext(999999)
		if !strings.Contains(result, "Unregistered") {
			t.Errorf("expected 'Unregistered' for unknown user, got: %q", result)
		}
	})

	t.Run("registered active user with group", func(t *testing.T) {
		// Create a group
		group := users.Group{
			ChatID:   -100123,
			Approved: true,
			Title:    "Test Fitness Group",
		}
		if err := db.DBCon.Create(&group).Error; err != nil {
			t.Fatalf("failed to create group: %v", err)
		}

		// Create a user with rank and associate with the group
		now := time.Now()
		user := users.User{
			TelegramUserID: 42042,
			Name:           "TestUser",
			Active:         true,
			Rank:           3,
			RankUpdatedAt:  &now,
			Groups:         []*users.Group{&group},
		}
		if err := db.DBCon.Create(&user).Error; err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		result := getUserContext(42042)

		if !strings.Contains(result, "Active") {
			t.Errorf("expected 'Active' status, got: %q", result)
		}
		if !strings.Contains(result, "Test Fitness Group") {
			t.Errorf("expected group name 'Test Fitness Group', got: %q", result)
		}
		if !strings.Contains(result, "Developing") {
			t.Errorf("expected rank 'Developing' (rank 3), got: %q", result)
		}
	})

	t.Run("registered inactive user without groups", func(t *testing.T) {
		now := time.Now()
		user := users.User{
			TelegramUserID: 42043,
			Name:           "InactiveUser",
			Active:         false,
			Rank:           1,
			RankUpdatedAt:  &now,
		}
		if err := db.DBCon.Create(&user).Error; err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		result := getUserContext(42043)

		if !strings.Contains(result, "Inactive") {
			t.Errorf("expected 'Inactive' status, got: %q", result)
		}
		if !strings.Contains(result, "Groups: None") {
			t.Errorf("expected 'Groups: None', got: %q", result)
		}
		if !strings.Contains(result, "Baby") {
			t.Errorf("expected rank 'Baby' (rank 1), got: %q", result)
		}
	})
}

func TestSupportGetSupportGroupChatID(t *testing.T) {
	var tests = []struct {
		name       string
		configVal  int64
		wantChatID int64
	}{
		{"configured chat ID", -1001234567890, -1001234567890},
		{"zero means not configured", 0, 0},
		{"positive chat ID", 987654321, 987654321},
		{"negative group ID", -100999, -100999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("support.group_chat_id", tt.configVal)

			got := getSupportGroupChatID()
			if got != tt.wantChatID {
				t.Errorf("getSupportGroupChatID() = %d, want %d", got, tt.wantChatID)
			}
		})
	}

	t.Cleanup(func() { viper.Reset() })
}

func TestSupportIsSupportGroupConfigured(t *testing.T) {
	var tests = []struct {
		name      string
		configVal int64
		want      bool
	}{
		{"configured with valid ID", -1001234567890, true},
		{"not configured (zero)", 0, false},
		{"configured with positive ID", 123456, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("support.group_chat_id", tt.configVal)

			got := isSupportGroupConfigured()
			if got != tt.want {
				t.Errorf("isSupportGroupConfigured() = %t, want %t", got, tt.want)
			}
		})
	}

	t.Cleanup(func() { viper.Reset() })
}

func TestSupportIsSupportGroupConfiguredNotSet(t *testing.T) {
	viper.Reset()
	if isSupportGroupConfigured() {
		t.Error("isSupportGroupConfigured() = true when no config is set, want false")
	}
	t.Cleanup(func() { viper.Reset() })
}

func TestSupportConstants(t *testing.T) {
	var tests = []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"redis prefix", supportRedisPrefix, "support:msg:"},
		{"state key", supportStateKey, "support"},
		{"state TTL (5 min)", supportStateTTL, 300},
		{"mapping TTL (7 days)", supportMappingTTL, 604800},
		{"cooldown TTL (60 sec)", supportCooldownTTL, 60},
		{"cooldown key prefix", supportCooldownKey, "support:cooldown:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if fmt.Sprint(tt.got) != fmt.Sprint(tt.want) {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}
