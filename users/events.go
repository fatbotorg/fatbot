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
	RejoinedGroupEventType       eventType = "rejoinedGroup" // New: user returned to the system
)

type Event struct {
	gorm.Model
	UserID  uint
	GroupID int64
	Event   eventType
}

func (user *User) registerEvent(kind eventType, groupId int64) error {
	db := db.DBCon
	event := Event{
		UserID:  user.ID,
		GroupID: groupId,
		Event:   kind,
	}
	return db.Create(&event).Error
}

func (user *User) GetEvents() (events []Event) {
	db := db.DBCon
	db.Where("user_id = ?", user.ID).Find(&events)
	return
}

func (user *User) RegisterBanEvent() error {
	return user.registerEvent(BanEventType, 0)
}

func (user *User) RegisterLastDayNotificationEvent() error {
	return user.registerEvent(LastDayNotificationEventType, 0)
}

func (user *User) RegisterWeeklyLeaderEvent(groupId int64) error {
	return user.registerEvent(WeeklyLeaderEventType, groupId)
}

func (user *User) RegisterWeeklyMessageRepliedEvent(groupId int64) error {
	return user.registerEvent(WeeklyMessageRepliedType, groupId)
}

// Register a "rejoin" event (user rejoined the system)
func (user *User) RegisterRejoinEvent() error {
	return user.registerEvent(RejoinedGroupEventType, 0)
}

// Check if the user replied to the weekly message in a specific group in the last 7 days
func (user *User) HasRepliedToWeeklyMessage(groupId int64) bool {
	db := db.DBCon
	var events []Event

	// First get the most recent weekly leader event
	var lastLeaderEvent Event
	db.Where("user_id = ? AND event = ? AND group_id = ?",
		user.ID, WeeklyLeaderEventType, groupId).
		Order("created_at DESC").
		First(&lastLeaderEvent)

	if lastLeaderEvent.ID == 0 {
		// No weekly leader event found, so they haven't replied
		return false
	}

	// Now check if there's a reply event after the last leader event
	db.Where("user_id = ? AND event = ? AND group_id = ? AND created_at > ?",
		user.ID, WeeklyMessageRepliedType, groupId, lastLeaderEvent.CreatedAt).
		Find(&events)

	return len(events) > 0
}

func (user *User) IsWeeklyLeaderInGroup(groupId int64) bool {
	db := db.DBCon
	var events []Event
	// Find all weekly leader events from this week for this group
	oneWeekAgo := time.Now().AddDate(0, 0, -7)
	db.Where("user_id = ? AND event = ? AND group_id = ? AND created_at > ?",
		user.ID, WeeklyLeaderEventType, groupId, oneWeekAgo).Find(&events)
	return len(events) > 0
}
