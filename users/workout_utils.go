package users

import "time"

func IsLastWorkoutOverdue(lastWorkout time.Time) (bool, int) {
	lastWorkoutDay := time.Date(
		lastWorkout.Year(),
		lastWorkout.Month(),
		lastWorkout.Day(),
		0, 0, 0, 0,
		lastWorkout.Location(),
	)
	currentDay := time.Date(
		time.Now().Year(),
		time.Now().Month(),
		time.Now().Day(),
		0, 0, 0, 0,
		time.Now().Location(),
	)

	daysDiff := int(currentDay.Sub(lastWorkoutDay).Hours() / 24)
	return daysDiff > 5, daysDiff
}
