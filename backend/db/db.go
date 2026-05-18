package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/floodroute/floodroute/backend/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta", host, user, password, dbname, port)
	
	var db *gorm.DB
	var err error
	
	for i := 0; i < 10; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("Failed to connect to database (attempt %d): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatal("Failed to connect to database after retries: ", err)
	}

	// Auto Migration
	err = db.AutoMigrate(&models.User{}, &models.Incident{})
	if err != nil {
		log.Printf("Failed to auto migrate: %v", err)
	}

	DB = db
	log.Println("Database connection established")
}
