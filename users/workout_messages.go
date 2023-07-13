package users

import (
	"math/rand"
	"time"
)

//	func GetRandomWorkoutMessage() string {
//		rand.Seed(time.Now().Unix()) // initialize global pseudo random generator
//		workoutMessages := []string{
//			"Amazing job! Keep up the good work.",
//			"You killed it! So proud of you.",
//			"Way to go! You're crushing your fitness goals.",
//			"Impressive! Keep pushing yourself.",
//			"Excellent work! Your dedication is inspiring.",
//			"Nice job! Your hard work is paying off.",
//			"You're a machine! Keep up the momentum.",
//			"Outstanding! You're making progress every day.",
//			"Bravo! Keep up the consistent effort.",
//			"You're a rockstar! Keep up the fantastic work.",
//			"Great job! You're really pushing yourself.",
//			"Spectacular! You're on the right track.",
//			"Fantastic work! You're making strides.",
//			"You're on fire! Keep up the hard work.",
//			"Incredible! You should be proud of yourself.",
//			"A+ work! You're getting stronger every day.",
//			"Superb job! Keep up the good habits.",
//			"Magnificent! Keep up the dedication.",
//			"Kudos! You're getting closer to your goals.",
//			"Incredible work! You're an inspiration.",
//			"Fantastic effort! Keep up the positive attitude.",
//			"You're a champion! Keep up the great work.",
//			"Phenomenal job! Your hard work is paying dividends.",
//			"Stellar! You're really motivated.",
//			"Superb! You're making progress every day.",
//			"Outstanding effort! Keep up the determination.",
//			"Superb work! You're getting better all the time.",
//			"Phenomenal! Keep up the strong will.",
//			"Fabulous! Keep up the enthusiasm.",
//			"Incredible work! You're an amazing person.",
//		}
//		return workoutMessages[rand.Intn(len(workoutMessages))]
//	}

func GetRandomStreakMessage() string {
	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator
	streakMessages := []string{
		"Keep up the streak, superhero!",
		"You're killing the streak, champ!",
		"Streaking! Keep pushing yourself!",
		"Streaking is your new hobby!",
		"Keep the streak alive, warrior!",
		"Keep up the streak, rockstar!",
		"Streaker alert! You're amazing!",
		"Keep the streak alive, legend!",
		"Keep up the streak, athlete!",
		"Streaking! Keep it up, superstar!",
		"Your streak is inspiring, warrior!",
		"Keep up the streak, fitness guru!",
		"Streaking! You got this, champ!",
		"Keep the streak going, legend!",
		"Streaking! Never give up, warrior!",
		"Keep up the streak, fitness freak!",
		"Streaking is your new lifestyle!",
		"Keep the streak alive, ironman!",
		"Streaking! Stay committed, superstar!",
		"Keep up the streak, fitness queen/king!",
	}
	return streakMessages[rand.Intn(len(streakMessages))]
}
