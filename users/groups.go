package users

import (
	"gorm.io/gorm"
)

type Group struct {
	gorm.Model
	ChatID   int64
	Approved bool
	Title    string
	Users    []*User `gorm:"many2many:user_groups;"`
}

func GetGroups() (groups []Group) {
	getDB().Find(&groups)
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
