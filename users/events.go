package users

import (
	"fatbot/db"

	"gorm.io/gorm"
)

type eventType string

const (
	BanEventType                 eventType = "ban"
	LastDayNotificationEventType eventType = "lastDayNotification"
	WeeklyLeaderEventType        eventType = "weeklyLeader"
)

type Event struct {
	gorm.Model
	UserID  uint
	Event   eventType
	GroupID uint
}

func (user *User) registerEvent(kind eventType, groupId uint) error {
	db := db.GetDB()
	event := Event{
		UserID:  user.ID,
		Event:   kind,
		GroupID: groupId,
	}
	return db.Create(&event).Error
}

func (user *User) GetEvents() (events []Event) {
	db.GetDB().Where("user_id = ?", user.ID).Find(&events)
	return
}
func (user *User) RegisterBanEvent(groupId uint) error {
	return user.registerEvent(BanEventType, groupId)
}

func (user *User) RegisterLastDayNotificationEvent(groupId uint) error {
	return user.registerEvent(LastDayNotificationEventType, groupId)
}

func (user *User) RegisterWeeklyLeaderEvent(groupId uint) error {
	return user.registerEvent(WeeklyLeaderEventType, groupId)
}
