package state

import (
	"fmt"
	"os"

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

func SetWithTTL(key, value string, ttl int) error {
	c, err := dial()
	if err != nil {
		log.Errorf("set dial err: %s", err)
		return err
	}
	defer c.Close()

	_, err = c.Do("SET", key, value, "EX", ttl)
	if err != nil {
		log.Errorf("setex err: %s", err)
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

func Get(key string) (string, error) {
	return get(key)
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

// SetNX atomically sets a key with TTL only if it does not already exist.
// Returns true if the key was set (lock acquired), false if it already existed.
func SetNX(key, value string, ttl int) (bool, error) {
	c, err := dial()
	if err != nil {
		log.Errorf("setnx dial err: %s", err)
		return false, err
	}
	defer c.Close()

	result, err := redis.String(c.Do("SET", key, value, "EX", ttl, "NX"))
	if err == redis.ErrNil {
		// Key already exists — lock not acquired
		return false, nil
	}
	if err != nil {
		log.Errorf("setnx err: %s", err)
		return false, err
	}
	return result == "OK", nil
}

// SetPendingPhoto stores a Telegram file ID as a pending workout photo for a user.
// The key is keyed by the user's Telegram ID and expires after 24 hours.
func SetPendingPhoto(telegramUserID int64, fileID string) error {
	key := fmt.Sprintf("photo:pending:%d", telegramUserID)
	return SetWithTTL(key, fileID, 86400) // 24h
}

// GetPendingPhoto retrieves the pending workout photo file ID for a user.
// Returns an error (redis.ErrNil) if none exists.
func GetPendingPhoto(telegramUserID int64) (string, error) {
	key := fmt.Sprintf("photo:pending:%d", telegramUserID)
	return get(key)
}

// ClearPendingPhoto removes the pending workout photo for a user.
func ClearPendingPhoto(telegramUserID int64) error {
	key := fmt.Sprintf("photo:pending:%d", telegramUserID)
	return ClearString(key)
}

func ClearString(key string) error {
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
