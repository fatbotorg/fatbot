package users

import (
	"fatbot/db"
	"time"

	"gorm.io/gorm"
)

type eventType string

const (
	BanEventType                 eventType = "ban"
	LastDayNotificationEventType eventType = "lastDayNotification"
	WeeklyLeaderEventType        eventType = "weeklyLeader"
	WeeklyWinnerMessageEventType eventType = "weeklyWinnerMessage"
)

type Event struct {
	gorm.Model
	UserID  uint
	Event   eventType
	Data    string // Adding Data field to store winner's message
	GroupID uint   // Adding GroupID to associate events with groups
}

func (user *User) registerEvent(kind eventType) error {
	db := db.DBCon
	event := Event{
		UserID: user.ID,
		Event:  kind,
	}
	return db.Create(&event).Error
}

func (user *User) registerEventWithData(kind eventType, data string, groupID uint) error {
	db := db.DBCon
	event := Event{
		UserID:  user.ID,
		Event:   kind,
		Data:    data,
		GroupID: groupID,
	}
	return db.Create(&event).Error
}

func (user *User) GetEvents() (events []Event) {
	db := db.DBCon
	db.Where("user_id = ?", user.ID).Find(&events)
	return
}

func (user *User) RegisterBanEvent() error {
	return user.registerEvent(BanEventType)
}

func (user *User) RegisterLastDayNotificationEvent() error {
	return user.registerEvent(LastDayNotificationEventType)
}

func (user *User) RegisterWeeklyLeaderEvent() error {
	return user.registerEvent(WeeklyLeaderEventType)
}

func (user *User) RegisterWeeklyWinnerMessageEvent(message string, groupID uint) error {
	return user.registerEventWithData(WeeklyWinnerMessageEventType, message, groupID)
}

// GetLatestWeeklyWinnerMessage gets the most recent weekly winner message for a group
func GetLatestWeeklyWinnerMessage(groupID uint) (string, string, time.Time, error) {
	db := db.DBCon
	var event Event

	// Order by created_at desc to get the most recent message
	if err := db.Where("event = ? AND group_id = ?", WeeklyWinnerMessageEventType, groupID).
		Order("created_at desc").
		Limit(1).
		Find(&event).Error; err != nil {
		return "", "", time.Time{}, err
	}

	if event.ID == 0 {
		return "", "", time.Time{}, nil
	}

	// Get the user's name
	var user User
	if err := db.First(&user, event.UserID).Error; err != nil {
		return "", "", time.Time{}, err
	}

	return user.GetName(), event.Data, event.CreatedAt, nil
}
