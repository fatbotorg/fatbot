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
	WeeklyMessageRepliedType     eventType = "weeklyMessageReplied"
)

type Event struct {
	gorm.Model
	UserID uint
	Event  eventType
}

func (user *User) registerEvent(kind eventType) error {
	db := db.DBCon
	event := Event{
		UserID: user.ID,
		Event:  kind,
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

func (user *User) RegisterWeeklyMessageRepliedEvent() error {
	return user.registerEvent(WeeklyMessageRepliedType)
}

func (user *User) HasRepliedToWeeklyMessage() bool {
	db := db.DBCon
	var events []Event
	// Find all events from this week
	oneWeekAgo := time.Now().AddDate(0, 0, -7)
	db.Where("user_id = ? AND event = ? AND created_at > ?", user.ID, WeeklyMessageRepliedType, oneWeekAgo).Find(&events)
	return len(events) > 0
}
