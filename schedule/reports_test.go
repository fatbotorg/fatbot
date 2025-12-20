package schedule

import (
	"fatbot/users"
	"testing"
	"time"
)

func TestCollectWeeklyStats(t *testing.T) {
	// This test is incomplete and needs a proper way to mock the database
	// or use an in-memory database.
	t.Skip("Skipping test due to lack of a mock database")

	// The following is a placeholder for the test logic
	group := users.Group{
		ChatID: 123,
		Users: []users.User{
			{
				ID:             1,
				TelegramUserID: 101,
				Name:           "User A",
				Workouts: []users.Workout{
					{CreatedAt: time.Now().AddDate(0, 0, -1)}, // 1 day ago
					{CreatedAt: time.Now().AddDate(0, 0, -8)}, // 8 days ago
				},
			},
			{
				ID:             2,
				TelegramUserID: 102,
				Name:           "User B",
				Workouts: []users.Workout{
					{CreatedAt: time.Now().AddDate(0, 0, -2)}, // 2 days ago
				},
			},
		},
	}

	stats := collectWeeklyStats(group)

	if len(stats) != 2 {
		t.Fatalf("Expected stats for 2 users, got %d", len(stats))
	}

	// This is a very basic assertion and needs to be improved with real data
	if stats[0].ThisWeekWorkouts != 1 {
		t.Errorf("Expected 1 workout for User A, got %d", stats[0].ThisWeekWorkouts)
	}
	if stats[1].ThisWeekWorkouts != 1 {
		t.Errorf("Expected 1 workout for User B, got %d", stats[1].ThisWeekWorkouts)
	}
}
