package migrations

import (
	"fatbot/db"
	"fatbot/users"

	"github.com/charmbracelet/log"
)

func ChatIdToGroupsMigration(user *users.User) {
	if len(user.Groups) == 0 {
		group, err := users.GetGroup(user.ChatID)
		if err != nil {
			log.Error(err)
		}
		db.GetDB().Model(&user).Association("Groups").Append(&group)
	}
	return
}

func CreateGroupsMigration() {
	usersList := users.GetUsers(-1)
	groups := users.GetGroupsWithUsers()
	for _, user := range usersList {
		if !inList(user.ChatID, groups) {
			groupObject := users.Group{
				ChatID:   user.ChatID,
				Approved: true,
				Title:    "",
				Users:    []users.User{user},
			}
			db.GetDB().Create(&groupObject)
			groups = users.GetGroupsWithUsers()
		} else {
			_, err := users.GetGroup(user.ChatID)
			if err != nil {
				log.Error(err)
			}
			// db.GetDB().Model(&group).Association("Users").Append(user)
			// db.GetDB().Model(&usersList).Association("Groups").Append(group)
		}
	}
}

func inList(chatId int64, groups []users.Group) bool {
	for _, group := range groups {
		if group.ChatID == chatId {
			return true
		}
	}
	return false
}

func WorkoutsToGroupsMigration() {
	var workouts []users.Workout

	if err := db.GetDB().Find(&workouts).Error; err != nil {
		log.Error(err)
		return
	}
	for _, workout := range workouts {
		if workout.GroupID == 0 {
			user, _ := users.GetUser(workout.UserID)
			group, _ := users.GetGroup(user.ChatID)
			db.GetDB().Model(&workout).Update("group_id", group.ID)
		}
	}
}
