package db

import (
	"log"
	"os"
	"time"

	_ "modrinth-mod-updater/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "go.uber.org/zap"
	_ "go.uber.org/zap/zapcore"
)

var DB *gorm.DB

// InitDatabase initializes the SQLite database connection and migrates models.
func InitDatabase(dbPath string) {
	var err error

	// Configure GORM logger
	newLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // Use standard log writer (os.Stdout)
		gormlogger.Config{
			SlowThreshold:             time.Second,   // Slow SQL threshold
			LogLevel:                  gormlogger.Warn, // Log level (Warn, Error, Info)
			IgnoreRecordNotFoundError: true,          // Ignore ErrRecordNotFound error
			ParameterizedQueries:      false,         // Log SQL queries with params
			Colorful:                  true,          // Enable color
		},
	)

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: newLogger, // Use the configured logger
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Auto-migrate the Mod and ModVersion schema
	err = DB.AutoMigrate(&Mod{}, &ModVersion{})
	if err != nil {
		log.Fatalf("failed to migrate database schema: %v", err)
	}
}
