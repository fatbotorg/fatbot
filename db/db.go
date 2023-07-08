package db

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		NowFunc: func() time.Time {
			return time.Now().In(location)
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	return db
}
