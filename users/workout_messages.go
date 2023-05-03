package users

import (
	"math/rand"
	"time"
)

func GetRandomWorkoutMessage() string {
	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator
	workoutMessages := []string{
		"Amazing job! Keep up the good work.",
		"You killed it! So proud of you.",
		"Way to go! You're crushing your fitness goals.",
		"Impressive! Keep pushing yourself.",
		"Excellent work! Your dedication is inspiring.",
		"Nice job! Your hard work is paying off.",
		"You're a machine! Keep up the momentum.",
		"Outstanding! You're making progress every day.",
		"Bravo! Keep up the consistent effort.",
		"You're a rockstar! Keep up the fantastic work.",
	}
	return workoutMessages[rand.Intn(len(workoutMessages))]
}
