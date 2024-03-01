package updates

import (
	"math/rand"
	"strings"
	"time"
)

type Emoji struct {
	String      string
	Description string
}

// a functions that for a given word in english check there's an available emoji
// and return it
// if not return an empty string
// Context from Function updates/emojis.go:findEmoji
func findReaction(labels []string) string {
	acceptedLables := map[string]string{
		"thumbs": "👍", "up": "👍", "thumbsup": "👍", "down": "👎", "heart": "❤",
		"love": "❤", "flame": "🔥", "fire": "🔥", "burn": "🔥", "inlove": "🥰",
		"hi": "👏", "hello": "👏", "wave": "👏", "smile": "😁", "grin": "😁",
		"think": "🤔", "wow": "🤯", "oh": "😱", "shock": "😱", "angry": "🤬",
		"sad": "😢", "party": "🎉", "stars": "🤩", "poop": "💩", "thanks": "🙏",
		"pray": "🙏", "ok": "👌", "peace": "🕊", "bird": "🕊", "clown": "🤡",
		"joke": "🤡", "tired": "🥱", "yawn": "🥱", "sick": "🥴", "fish": "🐳",
		"moon": "🌚", "hotdog": "🌭", "100": "💯", "lol": "🤣", "lightning": "⚡",
		"banana": "🍌", "trophy": "🏆", "strawberry": "🍓", "bottle": "🍾", "lips": "💋",
		"sleep": "😴", "cry": "😭", "ghost": "👻", "computer": "💻",
		"eyes": "👀", "pumpkin": "🎃", "angle": "😇", "frown": "😨", "hands": "🤝",
		"happy": "🤗", "salute": "🫡", "santa": "🎅", "tree": "🎄", "bug": "☃",
		"tounge": "🤪", "rock": "🗿", "stone": "🗿", "cool": "🆒", "ears": "🙉",
		"horse": "🦄", "kiss": "😘", "pill": "💊", "glasses": "😎", "sun": "😎",
		"shrug": "🤷",
	}
	for _, label := range labels {
		emoji, ok := acceptedLables[strings.ToLower(label)]
		if ok {
			return emoji
		}
	}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	keys := make([]string, 0, len(acceptedLables))
	for k := range acceptedLables {
		keys = append(keys, k)
	}
	randomKey := keys[rand.Intn(len(keys))] // select a random key
	selected := acceptedLables[randomKey]   // get the corresponding value
	return selected
}
