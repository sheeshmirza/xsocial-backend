package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/database"
	"xsocial-backend/internal/database/migrations"
)

func main() {
	flag.Parse()
	action := "up"
	if len(flag.Args()) > 0 {
		action = flag.Args()[0]
	}

	cfg := config.LoadConfig()
	mongoDB, err := database.ConnectMongoDB(cfg)
	if err != nil {
		log.Fatalf("[MigrateCLI] Failed to connect to MongoDB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	migrator := migrations.NewMigrator(mongoDB.Database)

	switch action {
	case "up":
		log.Println("[MigrateCLI] Executing pending migrations...")
		if err := migrator.ApplyAll(ctx); err != nil {
			log.Fatalf("[MigrateCLI] Migration up failed: %v", err)
		}
		log.Println("[MigrateCLI] All migrations completed successfully.")

	case "down", "rollback":
		log.Println("[MigrateCLI] Rolling back latest migration...")
		if err := migrator.RollbackLast(ctx); err != nil {
			log.Fatalf("[MigrateCLI] Rollback failed: %v", err)
		}

	case "status":
		statuses, err := migrator.Status(ctx)
		if err != nil {
			log.Fatalf("[MigrateCLI] Status check failed: %v", err)
		}
		fmt.Printf("\n%-8s %-35s %-10s %s\n", "VERSION", "NAME", "APPLIED", "APPLIED AT")
		fmt.Println("----------------------------------------------------------------------")
		for _, s := range statuses {
			appliedStr := "No"
			appliedAtStr := "-"
			if s.Applied {
				appliedStr = "Yes"
				appliedAtStr = s.AppliedAt.Format(time.RFC3339)
			}
			fmt.Printf("%-8d %-35s %-10s %s\n", s.Version, s.Name, appliedStr, appliedAtStr)
		}
		fmt.Println()

	case "seed":
		log.Println("[MigrateCLI] Seeding database with initial data...")
		summary, err := database.SeedInitialData(ctx, mongoDB.Database)
		if err != nil {
			log.Fatalf("[MigrateCLI] Seeding failed: %v", err)
		}
		fmt.Printf("\nSeeding Completed:\n - Users: %d\n - Brands: %d\n - Posts: %d\n - Handles: %d\n - Audit Logs: %d\n\n",
			summary.UsersCreated, summary.BrandsCreated, summary.PostsCreated, summary.HandlesCreated, summary.LogsCreated)

	default:
		fmt.Printf("Usage: %s [up|down|status|seed]\n", os.Args[0])
		os.Exit(1)
	}
}
