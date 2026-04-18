package updates

import (
	"fatbot/garmin"
	"testing"
)

func TestActivitiesFromGarminEntryNormalizesSummaryID(t *testing.T) {
	activities, err := activitiesFromGarminEntry(GarminEntry{
		SummaryId:          "abc123-detail",
		ActivityName:       "Morning Run",
		ActivityType:       "RUNNING",
		DurationInSeconds:  3600,
		StartTimeInSeconds: 1710000000,
		ActiveKilocalories: 420,
		AverageHeartRate:   150,
		DistanceInMeters:   10000,
	}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0].SummaryID != "abc123" {
		t.Fatalf("expected normalized summary ID abc123, got %s", activities[0].SummaryID)
	}
}

func TestLimitGarminActivitiesKeepsMostRecent(t *testing.T) {
	activities := limitGarminActivities([]garmin.ActivityData{
		{SummaryID: "older", StartTimeInSeconds: 100},
		{SummaryID: "newest", StartTimeInSeconds: 300},
		{SummaryID: "middle", StartTimeInSeconds: 200},
	}, 1)

	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0].SummaryID != "newest" {
		t.Fatalf("expected newest activity to remain, got %s", activities[0].SummaryID)
	}
}
