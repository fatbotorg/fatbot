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

var defaultRanks = []Rank{
	{"NonExistent", "🕳️", 0},
	{"Disastrous", "🚨", 36},
	{"Wretched", "", 73},
	{"Poor", "😬", 109},
	{"Weak", "😟", 146},
	{"Inadequate", "😐", 182},
	{"Passable", "🙂", 219},
	{"Solid", "👍", 255},
	{"Excellent", "✅", 292},
	{"Formidable", "💪", 328},
	{"Outstanding", "🔥", 365},
	{"Brilliant", "✨", 401},
	{"Magnificent", "🌟", 438},
	{"WorldClass", "🌍", 474},
	{"Supernatural", "👻", 511},
	{"Titanic", "🗿", 547},
	{"ExtraTerrestrial", "👽", 584},
	{"Mythical", "🧙‍♂️", 620},
	{"Magical", "🤙", 657},
	{"Utopian", "🧞", 693},
	{"Divine", "🕐", 730},
}

func getRanks() []Rank {
	result := make([]Rank, len(defaultRanks))
	copy(result, defaultRanks)
	return result
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

func GetNextRank(current Rank) (Rank, bool) {
	for i, rank := range defaultRanks {
		if rank.Name == current.Name && i+1 < len(defaultRanks) {
			return defaultRanks[i+1], true
		}
	}
	return Rank{}, false
}

func (user *User) updateRankIfNeeded() error {
	// Make sure RankUpdatedAt exists
	if err := user.EnsureRankUpdatedAtExists(); err != nil {
		return err
	}

	if user.RankUpdatedAt == nil {
		log.Debugf("User %s has no RankUpdatedAt – skipping rank calculation.", user.GetName())
		return nil
	}

	log.Debugf("Start Rank Calculation for User %s (ID: %d)", user.GetName(), user.ID)

	// Calculate effective days since RankUpdatedAt
	effectiveDays := int(time.Since(*user.RankUpdatedAt).Hours() / 24)
	log.Debugf("Effective days for user %s: %d", user.GetName(), effectiveDays)

	// Find the current rank
	currentRank, ok := GetRankByName(user.RankName)
	if !ok {
		// If no rank name but we have RankUpdatedAt, calculate the appropriate rank based on days
		ranks := getRanks()
		if user.RankName == "" {
			// Find the highest rank that matches the days threshold
			for i := len(ranks) - 1; i >= 0; i-- {
				rank := ranks[i]
				if effectiveDays >= rank.MinDays {
					currentRank = rank
					ok = true
					break
				}
			}
			if !ok {
				// If no rank matches, use the first rank
				currentRank = ranks[0]
			}
			log.Infof("Calculated initial rank '%s' for user %s based on %d days", currentRank.Name, user.GetName(), effectiveDays)
			user.RankName = currentRank.Name
			if err := db.DBCon.Save(&user).Error; err != nil {
				log.Errorf("Failed to save calculated rank for user %s: %v", user.GetName(), err)
				return err
			}
		} else {
			log.Warnf("Unknown current rank '%s' for user %s. Defaulting to first rank.", user.RankName, user.GetName())
			currentRank = ranks[0]
			user.RankName = currentRank.Name
			if err := db.DBCon.Save(&user).Error; err != nil {
				log.Errorf("Failed to save default rank for user %s: %v", user.GetName(), err)
				return err
			}
		}
		return nil
	}

	// Promote the user one rank if enough days have passed
	nextRank, ok := GetNextRank(currentRank)
	if !ok {
		log.Debugf("User %s already has the highest rank '%s'", user.GetName(), currentRank.Name)
		return nil
	}

	daysNeeded := nextRank.MinDays - currentRank.MinDays
	log.Debugf("Days needed to go from '%s' to '%s': %d", currentRank.Name, nextRank.Name, daysNeeded)

	if effectiveDays >= daysNeeded {
		log.Infof("Promoting user %s from '%s' to '%s'", user.GetName(), currentRank.Name, nextRank.Name)
		user.RankName = nextRank.Name
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

func (user *User) EnsureRankUpdatedAtExists() error {
	if user.RankUpdatedAt != nil {
		return nil
	}

	// Try to get the last rejoin event
	lastRejoin, err := user.GetLastRejoinEvent()
	if err == nil {
		user.RankUpdatedAt = &lastRejoin.CreatedAt
		if user.RankName == "" {
			user.RankName = getRanks()[0].Name
		}
		if err := db.DBCon.Save(&user).Error; err != nil {
			log.Errorf("Failed to save RankUpdatedAt after rejoin for user %s: %v", user.GetName(), err)
			return err
		}
		log.Infof("Initialized RankUpdatedAt for user %s to rejoin date: %s", user.GetName(), lastRejoin.CreatedAt.Format("2006-01-02"))
		return nil
	}

	// No rejoin found, fallback to first workout
	firstWorkout, err := user.getFirstWorkout()
	if err != nil {
		return nil // No workouts either
	}

	user.RankUpdatedAt = &firstWorkout.CreatedAt
	if user.RankName == "" {
		user.RankName = getRanks()[0].Name
	}
	if err := db.DBCon.Save(&user).Error; err != nil {
		log.Errorf("Failed to save RankUpdatedAt after workout for user %s: %v", user.GetName(), err)
		return err
	}

	log.Infof("Initialized RankUpdatedAt for user %s to first workout date: %s", user.GetName(), firstWorkout.CreatedAt.Format("2006-01-02"))
	return nil
}

// 🔄 Run rank update for all users in chatId 0
func UpdateAllUserRanks() {
	for _, user := range GetUsers(0) {
		err := user.updateRankIfNeeded()
		if err != nil {
			log.Errorf("Failed to update rank for user %s: %v", user.GetName(), err)
		}
	}
}
