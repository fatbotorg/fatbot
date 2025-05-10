package users

import (
	"fatbot/db"
	"time"

	"github.com/charmbracelet/log"
)

type Rank struct {
	Name    string
	Emoji   string
	MinDays int
}

var defaultRanks = map[int]Rank{
	1:  {"Baby", "🐥", 0},
	2:  {"Novice", "👶", 7},
	3:  {"Developing", "👦", 30},
	4:  {"Advancing", "🚀", 60},
	5:  {"Proficient", "✅", 90},
	6:  {"Competent", "🛠️", 150},
	7:  {"Capable", "👌", 210},
	8:  {"Solid", "🪨", 270},
	9:  {"Excellent", "🤩", 330},
	10: {"Formidable", "💪", 390},
	11: {"Outstanding", "🔥", 450},
	12: {"Brilliant", "✨", 510},
	13: {"Magnificent", "🌟", 570},
	14: {"WorldClass", "🌍", 630},
	15: {"Supernatural", "👻", 690},
	16: {"Titanic", "🗿", 750},
	17: {"ExtraTerrestrial", "👽", 810},
	18: {"Mythical", "🧙", 870},
	19: {"Magical", "🤙", 930},
	20: {"Utopian", "🧞", 990},
	21: {"Divine", "🕐", 1050},
}

func GetRanks() map[int]Rank {
	return defaultRanks
}

func GetRankByName(name string) (Rank, bool) {
	if name == "" {
		return Rank{}, false
	}
	for _, rank := range defaultRanks {
		if rank.Name == name {
			return rank, true
		}
	}
	return Rank{}, false
}

func (user *User) promoteRank() error {
	ranks := GetRanks()
	nextRank, ok := ranks[user.Rank+1]
	if !ok {
		log.Debugf("User %s already has the highest rank", user.GetName())
		return nil
	}

	currentRank := ranks[user.Rank]
	daysNeeded := nextRank.MinDays - currentRank.MinDays
	log.Debugf("Days needed to go from '%s' to '%s': %d",
		currentRank.Name, nextRank.Name, daysNeeded)

	effectiveDays := int(time.Since(*user.RankUpdatedAt).Hours() / 24)
	log.Debugf("Effective days for user %s: %d", user.GetName(), effectiveDays)

	if effectiveDays >= daysNeeded {
		log.Infof("Promoting user %s from '%s' to '%s'",
			user.GetName(), currentRank.Name, nextRank.Name)
		user.Rank++
		user.RankUpdatedAt = ptrTimeNow()
		if err := db.DBCon.Save(&user).Error; err != nil {
			log.Errorf("Failed to save promoted user %s: %v", user.GetName(), err)
			return err
		}
		log.Debugf("User %s rank updated successfully", user.GetName())
	} else {
		log.Debugf("User %s doesn't have enough days for promotion", user.GetName())
	}
	return nil
}

func (user *User) handleNeverRanked() error {
	log.Debugf("User %s has no RankUpdatedAt – setting default rank.",
		user.GetName())
	user.Rank = 1
	user.RankUpdatedAt = ptrTimeNow()
	if err := db.DBCon.Save(&user).Error; err != nil {
		log.Errorf("Failed to save user %s: %v", user.GetName(), err)
		return err
	}
	return nil
}

func (user *User) updateRank() error {
	// if user never updated, set defaults
	if user.RankUpdatedAt == nil {
		return user.handleNeverRanked()
	}

	// promote
	return user.promoteRank()
}

// Run rank update for all users
func UpdateAllUserRanks() {
	for _, user := range GetUsers(0) {
		if err := user.updateRank(); err != nil {
			log.Errorf("Failed to update rank for user %s: %v", user.GetName(), err)
		}
	}
}
