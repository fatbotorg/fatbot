package users

type Rank struct {
	Name    string
	Emoji   string
	MinDays int
}

var Ranks = []Rank{
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
