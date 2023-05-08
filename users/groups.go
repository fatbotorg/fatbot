package users

import (
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
	getDB().Preload("Users").Find(&groups)
	return
}

func GetGroup(chatId int64) (group Group, err error) {
	err = getDB().Where("chat_id = ?", chatId).Find(&group).Error
	return
}

func (group *Group) GetUsers() (users []User, err error) {
	err = getDB().Model(&group).Association("Users").Find(&users)
	return
}

func (group *Group) GetUserFixedNamesList() (userNames []string) {
	users, err := group.GetUsers()
	if err != nil {
		return nil
	}
	for _, user := range users {
		userNames = append(userNames, user.GetName())
	}
	return
}

func IsApprovedChatID(chatID int64) bool {
	db := getDB()
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
		user, err = user.LoadGroups()
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
	db := getDB()
	if group, err := GetGroup(chatId); err != nil {
		return err
	} else {
		db.Model(&user).Association("Groups").Append(group)
	}
	return nil
}
