package state

import (
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
