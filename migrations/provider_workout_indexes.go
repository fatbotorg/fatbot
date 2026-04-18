package migrations

import (
	"fatbot/db"

	"github.com/charmbracelet/log"
)

// AddProviderWorkoutUniqueIndexes adds partial unique indexes that prevent the
// same integration activity from being inserted twice for the same group.
func AddProviderWorkoutUniqueIndexes() {
	database := db.DBCon

	indexes := []struct {
		name  string
		query string
	}{
		{
			name: "idx_workouts_group_whoop_id",
			query: `
				CREATE UNIQUE INDEX IF NOT EXISTS idx_workouts_group_whoop_id
				ON workouts(group_id, whoop_id)
				WHERE whoop_id IS NOT NULL AND whoop_id != ''
			`,
		},
		{
			name: "idx_workouts_group_garmin_id",
			query: `
				CREATE UNIQUE INDEX IF NOT EXISTS idx_workouts_group_garmin_id
				ON workouts(group_id, garmin_id)
				WHERE garmin_id IS NOT NULL AND garmin_id != ''
			`,
		},
		{
			name: "idx_workouts_group_strava_id",
			query: `
				CREATE UNIQUE INDEX IF NOT EXISTS idx_workouts_group_strava_id
				ON workouts(group_id, strava_id)
				WHERE strava_id IS NOT NULL AND strava_id != ''
			`,
		},
	}

	for _, index := range indexes {
		result := database.Exec(index.query)
		if result.Error != nil {
			log.Error("Failed to create provider workout unique index", "index", index.name, "error", result.Error)
			continue
		}
		log.Info("Successfully ensured provider workout unique index", "index", index.name)
	}
}
