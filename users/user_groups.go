package users

import (
	"fatbot/db"
	"time"
)

// UserGroup is the explicit join table for the many2many relationship between User and Group.
// GORM already created this table implicitly. Adding CreatedAt lets us track when a user joined a group.
type UserGroup struct {
	UserID    uint `gorm:"primaryKey"`
	GroupID   uint `gorm:"primaryKey"`
	CreatedAt time.Time
}

// GetUserGroupJoinDate returns when a user joined a specific group.
// If the record has no created_at (legacy data), returns zero time.
func GetUserGroupJoinDate(userID uint, groupID uint) (time.Time, error) {
	db := db.DBCon
	var ug UserGroup
	err := db.Where("user_id = ? AND group_id = ?", userID, groupID).First(&ug).Error
	if err != nil {
		return time.Time{}, err
	}
	return ug.CreatedAt, nil
}
