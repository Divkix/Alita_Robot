package db

import (
	"context"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/divkix/Alita_Robot/alita/config"
)

var (
	MongoClient *mongo.Client
	DB          *mongo.Database
)

// isCliModeActive returns true if the program is running with CLI flags
// that should skip database initialization (--version, --health, -v).
func isCliModeActive() bool {
	if strings.HasSuffix(os.Args[0], ".test") || strings.Contains(os.Args[0], "/go-build") {
		return true
	}
	if len(os.Args) < 2 {
		return false
	}
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version", "-version", "-v", "--health", "-health":
			return true
		}
	}
	return false
}

func init() {
	if isCliModeActive() {
		return
	}
	if os.Getenv("MONGO_DB_URL") == "" {
		return
	}

	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dsn := config.AppConfig.MongoDBURL
	if dsn == "" {
		dsn = os.Getenv("MONGO_DB_URL")
	}

	clientOptions := options.Client().ApplyURI(dsn)

	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		MongoClient, err = mongo.Connect(ctx, clientOptions)
		if err == nil {
			err = MongoClient.Ping(ctx, readpref.Primary())
			if err == nil {
				break
			}
		}

		log.WithFields(log.Fields{
			"attempt": attempt + 1,
			"error":   err,
		}).Warning("[Database][Connection] Failed to connect to MongoDB, retrying...")

		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}

	if err != nil {
		log.Fatalf("[Database][Connection] Failed after %d attempts: %v", maxRetries, err)
	}

	// Extract database name from connection string or use default
	dbName := "alita_robot"
	if parts := strings.Split(dsn, "/"); len(parts) > 3 {
		if dbPart := strings.Split(parts[3], "?")[0]; dbPart != "" {
			dbName = dbPart
		}
	}

	DB = MongoClient.Database(dbName)
	log.Info("Connected to MongoDB database successfully!")
}

// Close closes the database connection gracefully.
func Close() error {
	if MongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return MongoClient.Disconnect(ctx)
	}
	return nil
}
