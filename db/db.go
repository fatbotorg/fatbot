package db

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DBCon *gorm.DB
)

func GetDB() *gorm.DB {
	timezone := viper.GetString("timezone")
	location, _ := time.LoadLocation(timezone)
	path := os.Getenv("DBPATH")
	if path == "" {
		path = "fat.db"
	}
	newLogger := logger.New(
		log.Default(),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: newLogger,
		NowFunc: func() time.Time {
			return time.Now().In(location)
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	return db
}
