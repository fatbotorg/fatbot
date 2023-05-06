package users

import "gorm.io/gorm"

type eventType string

const (
	Ban                 eventType = "ban"
	LastDayNotification eventType = "lastDayNotification"
	WeeklyLeader        eventType = "weeklyLeader"
)

type Event struct {
	gorm.Model
	UserID uint
	Event  eventType
}

func (user *User) RegisterEvent(kind eventType) {
	db := getDB()
	event := Event{
		UserID: user.ID,
		Event:  kind,
	}
	db.Create(&event)
}
