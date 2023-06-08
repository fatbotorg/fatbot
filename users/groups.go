package users

import (
	"fatbot/db"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

type Group struct {
	gorm.Model
	ChatID   int64
	Approved bool
	Title    string
	Users    []User `gorm:"many2many:user_groups;"`
	Workouts []Workout
}

func GetGroupsWithUsers() (groups []Group) {
	db := db.DBCon
	db.Preload("Users", "active = ?", true).Find(&groups)
	return
}

func GetGroups() (groups []Group) {
	db := db.DBCon
	db.Find(&groups)
	return
}

func GetGroupWithUsers(chatId int64) (group Group) {
	db := db.DBCon
	db.Preload("Users", "active = ?", true).Where("chat_id = ?", chatId).Find(&group)
	return
}

func GetGroupWithInactiveUsers(chatId int64) (group Group) {
	db := db.DBCon
	db.Preload("Users", "active = ?", false).Where("chat_id = ?", chatId).Find(&group)
	return
}

func GetGroup(chatId int64) (group Group, err error) {
	db := db.DBCon
	err = db.Where("chat_id = ?", chatId).Find(&group).Error
	return
}

func (group *Group) GetUsers() (users []User, err error) {
	db := db.DBCon
	err = db.Model(&group).Association("Users").Find(&users)
	return
}

func (group *Group) GetUserFixedNamesList() (userNames []string) {
	for _, user := range group.Users {
		userNames = append(userNames, user.GetName())
	}
	return
}

func IsApprovedChatID(chatID int64) bool {
	db := db.DBCon
	var group Group
	result := db.Where("chat_id = ?", chatID).Find(&group)
	if result.RowsAffected == 0 {
		return false
	}
	return group.Approved
}

func (user *User) IsInGroup(chatId int64) bool {
	var err error
	if len(user.Groups) == 0 {
		err = user.LoadGroups()
		if err != nil {
			log.Error(err)
		}
	}
	for _, group := range user.Groups {
		if group.ChatID == chatId {
			return true
		}
	}
	return false
}

func (user *User) RegisterInGroup(chatId int64) error {
	db := db.DBCon
	if group, err := GetGroup(chatId); err != nil {
		return err
	} else {
		if err := db.Model(&user).Association("Groups").Append(&group); err != nil {
			return err
		}
	}
	return nil
}
