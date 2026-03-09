package users

import (
	"fatbot/db"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

type Group struct {
	gorm.Model
	ChatID              int64
	Approved            bool
	Title               string
	BestAverageWorkouts float64 `gorm:"default:0"`
	Slug                string
	CreatorID           int64
	Autonomous          bool
	Users               []User `gorm:"many2many:user_groups;"`
	Admins              []User `gorm:"many2many:groups_admins;"`
	Workouts            []Workout
}

func CreateGroup(chatId int64, title string) error {
	db := db.DBCon
	slug := GenerateUniqueSlug(title)
	group := Group{
		ChatID:   chatId,
		Approved: true,
		Title:    title,
		Slug:     slug,
	}
	return db.Create(&group).Error
}

// CreateAutonomousGroup creates a new group with autonomous settings, slug, and creator tracking.
func CreateAutonomousGroup(chatId int64, title string, creatorTelegramID int64) (Group, error) {
	db := db.DBCon
	slug := GenerateUniqueSlug(title)
	group := Group{
		ChatID:     chatId,
		Approved:   true,
		Title:      title,
		Slug:       slug,
		CreatorID:  creatorTelegramID,
		Autonomous: true,
	}
	err := db.Create(&group).Error
	return group, err
}

func GetGroupsWithUsers() (groups []Group) {
	db := db.DBCon
	db.Where("approved = ?", true).Preload("Users", "active = ?", true).Find(&groups)
	return
}

func GetGroups() (groups []Group) {
	db := db.DBCon
	db.Where("approved = ?", true).Find(&groups)
	return
}

func GetGroupsWithInsta() []Group {
	var groups []Group
	db.DBCon.Model(&Group{}).
		Joins("JOIN user_groups ON user_groups.group_id = groups.id").
		Joins("JOIN users ON users.id = user_groups.user_id").
		Joins("JOIN workouts ON workouts.user_id = users.id").
		Where("users.instagram_handle != ? AND workouts.photo_file_id != ?", "", "").
		Group("groups.id").
		Find(&groups)
	return groups
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
		// Check if already in group
		var count int64
		db.Model(&UserGroup{}).Where("user_id = ? AND group_id = ?", user.ID, group.ID).Count(&count)
		if count > 0 {
			return nil // Already registered
		}
		// Create explicit join record with timestamp
		ug := UserGroup{
			UserID:    user.ID,
			GroupID:   group.ID,
			CreatedAt: time.Now(),
		}
		if err := db.Create(&ug).Error; err != nil {
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

// UpdateBestAverageIfHigher compares current average with the stored best and updates if higher.
// Returns whether this is a new best, the previous best value, and any error.
// If previousBest is 0, this is the first recorded week.
func (group *Group) UpdateBestAverageIfHigher(currentAverage float64) (isNewBest bool, previousBest float64, err error) {
	previousBest = group.BestAverageWorkouts
	if currentAverage > group.BestAverageWorkouts {
		db := db.DBCon
		err = db.Model(group).Update("best_average_workouts", currentAverage).Error
		if err != nil {
			return false, previousBest, err
		}
		return true, previousBest, nil
	}
	return false, previousBest, nil
}

// GetGroupBySlug retrieves a group by its unique slug identifier.
func GetGroupBySlug(slug string) (group Group, err error) {
	db := db.DBCon
	if err = db.Where("slug = ?", slug).First(&group).Error; err != nil {
		return
	}
	return
}

// GenerateSlug converts a title to a URL-safe slug (lowercase alphanumeric only).
func GenerateSlug(title string) string {
	slug := strings.ToLower(title)
	var result []byte
	for _, c := range []byte(slug) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		result = []byte("group")
	}
	return string(result)
}

// GenerateUniqueSlug creates a slug from the title and ensures it doesn't collide
// with existing slugs by appending a numeric suffix if needed.
// The DB uniqueIndex on Slug is the final safety net against race conditions.
func GenerateUniqueSlug(title string) string {
	db := db.DBCon
	base := GenerateSlug(title)
	slug := base
	var count int64
	for i := 0; i < 100; i++ {
		db.Model(&Group{}).Where("slug = ?", slug).Count(&count)
		if count == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s%d", base, time.Now().UnixNano()%100000)
	}
	// Fallback: use timestamp to guarantee uniqueness
	return fmt.Sprintf("%s%d", base, time.Now().UnixNano())
}

// DeactivateGroupUsers sets all users in a group to inactive (for that group only).
func DeactivateGroupUsers(chatId int64) error {
	db := db.DBCon
	group, err := GetGroup(chatId)
	if err != nil {
		return err
	}
	users, err := group.GetUsers()
	if err != nil {
		return err
	}
	for _, user := range users {
		// Check if user is in any OTHER active group
		var otherGroupCount int64
		db.Model(&UserGroup{}).
			Joins("JOIN groups ON groups.id = user_groups.group_id").
			Where("user_groups.user_id = ? AND groups.id != ? AND groups.approved = ?", user.ID, group.ID, true).
			Count(&otherGroupCount)
		if otherGroupCount == 0 {
			// User is only in this group — deactivate them
			db.Model(&user).Update("active", false)
		}
	}
	return nil
}

// ClearGroupCreator resets the creator_id so the user's group slot is freed.
func ClearGroupCreator(chatId int64) error {
	db := db.DBCon
	return db.Model(&Group{}).Where("chat_id = ?", chatId).Update("creator_id", 0).Error
}

// UpdateGroupApproved sets the Approved flag for a group identified by its Telegram chat ID.
func UpdateGroupApproved(chatId int64, approved bool) error {
	db := db.DBCon
	return db.Model(&Group{}).Where("chat_id = ?", chatId).Update("approved", approved).Error
}

// GroupExistsByChatID checks whether a group with the given Telegram chat ID already exists.
func GroupExistsByChatID(chatId int64) bool {
	db := db.DBCon
	var count int64
	db.Model(&Group{}).Where("chat_id = ?", chatId).Count(&count)
	return count > 0
}

// CountAutonomousGroupsByCreator returns how many active autonomous groups a user has created.
func CountAutonomousGroupsByCreator(creatorTelegramID int64) int64 {
	db := db.DBCon
	var count int64
	db.Model(&Group{}).Where("creator_id = ? AND autonomous = ? AND approved = ?", creatorTelegramID, true, true).Count(&count)
	return count
}
