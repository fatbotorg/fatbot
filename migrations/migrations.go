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
		db.Model(&user).Association("Workouts").Append(workout)
		db.GetDB().Model(&user).Association("Groups").Append(group)
	}
	return
}
