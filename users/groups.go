package users

import (
	"fatbot/db"
	"fmt"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

type Group struct {
	gorm.Model
	ChatID   int64
	Approved bool
	Title    string
	Users    []User `gorm:"many2many:user_groups;"`
	Admins   []User `gorm:"many2many:groups_admins;"`
	Workouts []Workout
}

func CreateGroup(chatId int64, title string) error {
	db := db.DBCon
	group := Group{
		ChatID:   chatId,
		Approved: true,
		Title:    title,
	}
	return db.Create(&group).Error
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

func GetManagedGroups(adminUserId int64) (groups []Group) {
	adminuser, err := GetUserById(adminUserId)
	if err != nil {
		log.Error(err)
		return []Group{}
	}
	adminuser.loadManagedGroups()
	for _, group := range adminuser.GroupsAdmin {
		groups = append(groups, *group)
	}
	return groups
}

func GetGroupByTitle(title string) (group Group, err error) {
	db := db.DBCon
	if err = db.Where("title = ?", title).Find(&group).Error; err != nil {
		return
	}
	if group.Title == "" {
		err = fmt.Errorf("could not find group %s", title)
	}
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

func GetGroupWithAdmins(chatId int64) (Group, error) {
	if group, err := GetGroup(chatId); err != nil {
		return Group{}, err
	} else {
		if err := group.loadGroupAdmins(); err != nil {
			return Group{}, err
		} else {
			return group, nil
		}
	}
}

func (group *Group) loadGroupAdmins() error {
	db := db.DBCon
	err := db.Preload("Admins").Find(&group).Error
	if err != nil {
		return err
	}
	return nil
}

// GetGroupsWithUserCounts returns all groups with their user counts (active and total)
func GetGroupsWithUserCounts() (groupsInfo []map[string]interface{}) {
	db := db.DBCon
	var groups []Group

	// Get all groups
	db.Find(&groups)

	for _, group := range groups {
		// Load all users for this group
		var activeUsers []User
		var inactiveUsers []User

		db.Model(&group).Association("Users").Find(&activeUsers, "active = ?", true)
		db.Model(&group).Association("Users").Find(&inactiveUsers, "active = ?", false)

		groupInfo := map[string]interface{}{
			"group":         group,
			"activeCount":   len(activeUsers),
			"inactiveCount": len(inactiveUsers),
			"totalCount":    len(activeUsers) + len(inactiveUsers),
		}

		groupsInfo = append(groupsInfo, groupInfo)
	}

	return
}
