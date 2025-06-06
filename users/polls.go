package users

import (
	"fatbot/db"

	"gorm.io/gorm"
)

type WorkoutDisputePoll struct {
	gorm.Model
	PollID       string
	GroupID      uint
	TargetUserID uint
	WorkoutID    uint
	MessageID    int
}

func CreateWorkoutDisputePoll(pollID string, groupID, targetUserID, workoutID uint, messageID int) error {
	db := db.DBCon
	poll := WorkoutDisputePoll{
		PollID:       pollID,
		GroupID:      groupID,
		TargetUserID: targetUserID,
		WorkoutID:    workoutID,
		MessageID:    messageID,
	}
	return db.Create(&poll).Error
}

func GetWorkoutDisputePoll(pollID string) (WorkoutDisputePoll, error) {
	db := db.DBCon
	var poll WorkoutDisputePoll
	err := db.Where("poll_id = ?", pollID).First(&poll).Error
	return poll, err
}

func (poll *WorkoutDisputePoll) GetTargetUser() (User, error) {
	return GetUser(poll.TargetUserID)
}

func (poll *WorkoutDisputePoll) GetWorkout() (Workout, error) {
	db := db.DBCon
	var workout Workout
	err := db.First(&workout, poll.WorkoutID).Error
	return workout, err
}

func (poll *WorkoutDisputePoll) GetGroup() (Group, error) {
	return GetGroupByID(poll.GroupID)
}

func GetGroupByID(id uint) (Group, error) {
	db := db.DBCon
	var group Group
	err := db.First(&group, id).Error
	return group, err
}
