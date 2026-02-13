package garmin

import (
	"encoding/json"
	"testing"
)

func TestActivityParsing(t *testing.T) {
	jsonData := `[
		{
			"summaryId": "activity_123",
			"activityId": 123456,
			"activityName": "Morning Run",
			"activityType": "RUNNING",
			"startTimeInSeconds": 1700000000,
			"durationInSeconds": 3600,
			"calories": 500.5,
			"distanceInMeters": 10000,
			"averageHeartRateInBeatsPerMinute": 150
		}
	]`

	var activities []ActivityData
	err := json.Unmarshal([]byte(jsonData), &activities)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(activities) != 1 {
		t.Fatalf("Expected 1 activity, got %d", len(activities))
	}

	if activities[0].SummaryID != "activity_123" {
		t.Errorf("Expected summaryId activity_123, got %s", activities[0].SummaryID)
	}

	if activities[0].Calories != 500.5 {
		t.Errorf("Expected calories 500.5, got %f", activities[0].Calories)
	}
}
