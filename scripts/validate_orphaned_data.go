package main

import (
	"fmt"
	"log"
	"os"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
)

type OrphanReport struct {
	Table string
	Count int64
	Issue string
	SQL   string
}

func main() {
	// Load configuration
	if _, err := config.LoadConfig(); err != nil {
		log.Fatalf("[Validation] Failed to load configuration: %v", err)
	}

	// Initialize database
	if db.DB == nil {
		log.Fatal("[Validation] Database not initialized. Set DATABASE_URL environment variable.")
	}

	fmt.Println("🔍 Database Orphaned Data Validation")
	fmt.Println("=====================================")

	reports := []OrphanReport{}

	// Check for orphaned admin records
	var adminCount int64
	db.DB.Table("admin").Where("chat_id NOT IN (SELECT chat_id FROM chats)").Count(&adminCount)
	if adminCount > 0 {
		reports = append(reports, OrphanReport{
			Table: "admin",
			Count: adminCount,
			Issue: "Records with non-existent chat_id",
			SQL:   "DELETE FROM admin WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		})
	}

	// Check for orphaned antiflood_settings records
	var antifloodCount int64
	db.DB.Table("antiflood_settings").Where("chat_id NOT IN (SELECT chat_id FROM chats)").Count(&antifloodCount)
	if antifloodCount > 0 {
		reports = append(reports, OrphanReport{
			Table: "antiflood_settings",
			Count: antifloodCount,
			Issue: "Records with non-existent chat_id",
			SQL:   "DELETE FROM antiflood_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		})
	}

	// Check for orphaned warns_users records
	var warnsUsersCount int64
	db.DB.Table("warns_users").Where("chat_id NOT IN (SELECT chat_id FROM chats)").Count(&warnsUsersCount)
	if warnsUsersCount > 0 {
		reports = append(reports, OrphanReport{
			Table: "warns_users",
			Count: warnsUsersCount,
			Issue: "Records with non-existent chat_id",
			SQL:   "DELETE FROM warns_users WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		})
	}

	// Check for orphaned warns_users records (user side)
	var warnsUserCount int64
	db.DB.Table("warns_users").Where("user_id NOT IN (SELECT user_id FROM users)").Count(&warnsUserCount)
	if warnsUserCount > 0 {
		reports = append(reports, OrphanReport{
			Table: "warns_users",
			Count: warnsUserCount,
			Issue: "Records with non-existent user_id",
			SQL:   "DELETE FROM warns_users WHERE user_id NOT IN (SELECT user_id FROM users);",
		})
	}

	// Display results
	if len(reports) == 0 {
		fmt.Println("✅ No orphaned records found - database is clean!")
		os.Exit(0)
	}

	fmt.Printf("\n❌ Found %d types of orphaned records:\n\n", len(reports))
	for i, report := range reports {
		fmt.Printf("%d. Table: %s\n", i+1, report.Table)
		fmt.Printf("   Orphaned Records: %d\n", report.Count)
		fmt.Printf("   Issue: %s\n", report.Issue)
		fmt.Printf("   Cleanup SQL:\n   %s\n\n", report.SQL)
	}

	fmt.Println("⚠️  WARNING: Orphaned records detected!")
	fmt.Println("   Before deploying foreign key constraints, you must:")
	fmt.Println("   1. Review the orphaned records above")
	fmt.Println("   2. Run the cleanup SQL in a transaction")
	fmt.Println("   3. Re-run this validation script to confirm")
	fmt.Println("\n   Example cleanup command:")
	fmt.Println("   psql $DATABASE_URL -c \"BEGIN; <sql_above>; COMMIT;\"")

	os.Exit(1)
}
