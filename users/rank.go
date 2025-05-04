package users

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

func GetRanks() []Rank {
	result := make([]Rank, len(defaultRanks))
	copy(result, defaultRanks)
	return result
}

func GetRankByName(name string) (Rank, bool) {
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
