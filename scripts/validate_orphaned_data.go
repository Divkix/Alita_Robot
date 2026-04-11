package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/divkix/Alita_Robot/alita/db"
)

type OrphanReport struct {
	Table string
	Count int64
	Issue string
	SQL   string
}

type orphanCheck struct {
	table     string
	condition string
	issue     string
	cleanup   string
}

func main() {
	// Database initialization happens in db package init.
	if db.DB == nil {
		log.Fatal("[Validation] Database not initialized. Set DATABASE_URL environment variable.")
	}

	fmt.Println("🔍 Database Orphaned Data Validation")
	fmt.Println(strings.Repeat("=", 37))

	// Keep this list in sync with STEP 1 orphan cleanup in
	// migrations/20250805204145_add_foreign_key_relations.sql.
	checks := []orphanCheck{
		{
			table:     "admin",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM admin WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "antiflood_settings",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM antiflood_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "blacklists",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM blacklists WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "channels",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM channels WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "channels",
			condition: "channel_id IS NOT NULL AND channel_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent channel_id",
			cleanup: "UPDATE channels SET channel_id = NULL WHERE channel_id IS NOT NULL " +
				"AND channel_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "connection_settings",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM connection_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "disable",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM disable WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "filters",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM filters WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "greetings",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM greetings WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "locks",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM locks WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "notes",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM notes WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "notes_settings",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM notes_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "pins",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM pins WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "report_chat_settings",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM report_chat_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "rules",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM rules WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "warns_settings",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "Records with non-existent chat_id",
			cleanup:   "DELETE FROM warns_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);",
		},
		{
			table:     "devs",
			condition: "user_id NOT IN (SELECT user_id FROM users)",
			issue:     "Records with non-existent user_id",
			cleanup:   "DELETE FROM devs WHERE user_id NOT IN (SELECT user_id FROM users);",
		},
		{
			table:     "report_user_settings",
			condition: "user_id NOT IN (SELECT user_id FROM users)",
			issue:     "Records with non-existent user_id",
			cleanup:   "DELETE FROM report_user_settings WHERE user_id NOT IN (SELECT user_id FROM users);",
		},
		{
			table: "chat_users",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats) OR " +
				"user_id NOT IN (SELECT user_id FROM users)",
			issue: "Records with non-existent chat_id or user_id",
			cleanup: "DELETE FROM chat_users WHERE chat_id NOT IN (SELECT chat_id FROM chats) " +
				"OR user_id NOT IN (SELECT user_id FROM users);",
		},
		{
			table: "connection",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats) OR " +
				"user_id NOT IN (SELECT user_id FROM users)",
			issue: "Records with non-existent chat_id or user_id",
			cleanup: "DELETE FROM connection WHERE chat_id NOT IN (SELECT chat_id FROM chats) " +
				"OR user_id NOT IN (SELECT user_id FROM users);",
		},
		{
			table: "warns_users",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats) OR " +
				"user_id NOT IN (SELECT user_id FROM users)",
			issue: "Records with non-existent chat_id or user_id",
			cleanup: "DELETE FROM warns_users WHERE chat_id NOT IN (SELECT chat_id FROM chats) " +
				"OR user_id NOT IN (SELECT user_id FROM users);",
		},
	}

	reports := make([]OrphanReport, 0, len(checks))
	for _, check := range checks {
		var count int64
		err := db.DB.Table(check.table).Where(check.condition).Count(&count).Error
		if err != nil {
			log.Fatalf("[Validation] Failed to query %s: %v", check.table, err)
		}
		if count == 0 {
			continue
		}

		reports = append(reports, OrphanReport{
			Table: check.table,
			Count: count,
			Issue: check.issue,
			SQL:   check.cleanup,
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
	fmt.Println("\n   ⚠️  CRITICAL: Always run cleanup in transactions for safety!")
	fmt.Println("\n   Example cleanup transaction:")
	fmt.Println("   psql \"$DATABASE_URL\" << 'EOF'")
	fmt.Println("   BEGIN;")
	for _, report := range reports {
		fmt.Printf("   %s\n", report.SQL)
	}
	fmt.Println("   COMMIT;")
	fmt.Println("   EOF")

	os.Exit(1)
}
