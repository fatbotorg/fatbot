package users

import "gorm.io/gorm"

type eventType string

const (
	BanEventType                 eventType = "ban"
	LastDayNotificationEventType eventType = "lastDayNotification"
	WeeklyLeaderEventType        eventType = "weeklyLeader"
)

type Event struct {
	gorm.Model
	UserID uint
	Event  eventType
}

func (user *User) registerEvent(kind eventType) error {
	db := getDB()
	event := Event{
		UserID: user.ID,
		Event:  kind,
	}
	return db.Create(&event).Error
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
