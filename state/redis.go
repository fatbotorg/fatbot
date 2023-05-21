package state

import (
	"github.com/charmbracelet/log"
	"github.com/gomodule/redigo/redis"
)

func dial() (redis.Conn, error) {
	connection, err := redis.Dial("tcp", ":6379")
	if err != nil {
		log.Error("SHIT")
	}
	return connection, nil
}

func set(key, value string) error {
	c, err := dial()
	if err != nil {
		return err
	}
	defer c.Close()

	_, err = c.Do("SET", key, value)
	if err != nil {
		return err
	}
	return nil
}

func get(key string) (string, error) {
	c, err := dial()
	if err != nil {
		return "", err
	}
	s, err := redis.String(c.Do("GET", key))
	if err != nil {
		return "", err
	}
	return s, nil
}

func clear(key string) error {
	log.Debug(key)
	c, err := dial()
	if err != nil {
		return err
	}
	_, err = redis.String(c.Do("DEL", key))
	log.Debug(err)
	return err
}
