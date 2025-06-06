package state

import (
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/gomodule/redigo/redis"
)

func dial() (redis.Conn, error) {
	var connection redis.Conn
	var err error
	if os.Getenv("REDIS_ADDR") == "" {
		connection, err = redis.Dial("tcp", ":6379")
	} else {
		connection, err = redis.DialURL(
			os.Getenv("REDIS_ADDR"),
		)
	}
	if err != nil {
		log.Fatal("cannot find redis!")
	}
	return connection, nil
}

func set(key, value string) error {
	c, err := dial()
	if err != nil {
		log.Errorf("set dial err: %s", err)
		return err
	}
	defer c.Close()

	_, err = c.Do("SET", key, value)
	if err != nil {
		log.Errorf("set err: %s", err)
		return err
	}
	return nil
}

func get(key string) (string, error) {
	c, err := dial()
	if err != nil {
		log.Errorf("get dial err: %s", err)
		return "", err
	}
	s, err := redis.String(c.Do("GET", key))
	if err != nil {
		return "", err
	}
	return s, nil
}

func clear(key int64) error {
	c, err := dial()
	if err != nil {
		log.Errorf("del dial err: %s", err)
		return err
	}
	_, err = redis.Int64(c.Do("DEL", key))
	if err != nil {
		log.Errorf("del err: %s", err)
		return err
	}
	return nil
}

func clearString(key string) error {
	c, err := dial()
	if err != nil {
		log.Errorf("del dial err: %s", err)
		return err
	}
	_, err = c.Do("DEL", key)
	if err != nil {
		log.Errorf("del err: %s", err)
		return err
	}
	return nil
}

// PollChatMapping handles Redis operations for poll-to-chat mappings
type PollChatMapping struct{}

// StorePollChat stores the mapping between a poll ID and its chat ID
func (p *PollChatMapping) StorePollChat(pollID string, chatID int64) error {
	key := fmt.Sprintf("poll:%s", pollID)
	return set(key, fmt.Sprintf("%d", chatID))
}

// GetPollChat retrieves the chat ID for a given poll ID
func (p *PollChatMapping) GetPollChat(pollID string) (int64, error) {
	key := fmt.Sprintf("poll:%s", pollID)
	chatIDStr, err := get(key)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(chatIDStr, 10, 64)
}

// ClearPollChat removes the poll-to-chat mapping
func (p *PollChatMapping) ClearPollChat(pollID string) error {
	key := fmt.Sprintf("poll:%s", pollID)
	return clearString(key)
}

// NewPollChatMapping creates a new PollChatMapping instance
func NewPollChatMapping() *PollChatMapping {
	return &PollChatMapping{}
}

// Global instance for use across packages
var PollMapping = NewPollChatMapping()
