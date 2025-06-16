package users

import (
	"fatbot/db"

	"gorm.io/gorm"
)

// WorkoutDisputePoll represents a poll created to dispute a workout
type WorkoutDisputePoll struct {
	gorm.Model
	PollID    string `gorm:"uniqueIndex"`
	GroupID   uint
	UserID    uint
	WorkoutID uint
	MessageID int
}

// CreateWorkoutDisputePoll creates a new workout dispute poll record
func CreateWorkoutDisputePoll(pollID string, groupID, userID, workoutID uint, messageID int) error {
	db := db.DBCon
	poll := WorkoutDisputePoll{
		PollID:    pollID,
		GroupID:   groupID,
		UserID:    userID,
		WorkoutID: workoutID,
		MessageID: messageID,
	}
	return db.Create(&poll).Error
}

// GetWorkoutDisputePoll retrieves a workout dispute poll by poll ID
func GetWorkoutDisputePoll(pollID string) (*WorkoutDisputePoll, error) {
	db := db.DBCon
	var poll WorkoutDisputePoll
	if err := db.Where("poll_id = ?", pollID).First(&poll).Error; err != nil {
		return nil, err
	}
	return &poll, nil
}

// GetGroup retrieves the group associated with this poll
func (poll *WorkoutDisputePoll) GetGroup() (*Group, error) {
	return GetGroupByID(poll.GroupID)
}

// GetTargetUser retrieves the user whose workout is being disputed
func (poll *WorkoutDisputePoll) GetTargetUser() (*User, error) {
	user, err := GetUser(poll.UserID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetWorkout retrieves the workout being disputed
func (poll *WorkoutDisputePoll) GetWorkout() (*Workout, error) {
	db := db.DBCon
	var workout Workout
	if err := db.First(&workout, poll.WorkoutID).Error; err != nil {
		return nil, err
	}
	return &workout, nil
}

// GetUserByUsername retrieves a user by their username
func GetUserByUsername(username string) (*User, error) {
	db := db.DBCon
	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetGroupByID retrieves a group by its ID
func GetGroupByID(groupID uint) (*Group, error) {
	db := db.DBCon
	var group Group
	if err := db.First(&group, groupID).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// // IsGroupAdmin checks if a user is an admin of a specific group
// func (user *User) IsGroupAdmin(groupID uint) bool {
// 	db := db.DBCon
//
// 	// Load the user's managed groups
// 	if err := db.Preload("GroupsAdmin").Find(&user).Error; err != nil {
// 		return false
// 	}
//
// 	// Check if the user is admin of the specific group
// 	for _, adminGroup := range user.GroupsAdmin {
// 		if adminGroup.ID == groupID {
// 			return true
// 		}
// 	}
//
// 	return false
// }
