package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/motifengine/motifengine/internal/config"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:]); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || (args[0] != "up" && args[0] != "down") {
		return fmt.Errorf("usage: migrate [up|down]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if args[0] == "up" {
		return up(ctx, conn)
	}
	return down(ctx, conn)
}

func up(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	for _, migration := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26} {
		var applied int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = $1`, migration).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		if err := apply(ctx, conn, migration, "up"); err != nil {
			return err
		}
	}
	return nil
}

func down(ctx context.Context, conn *pgx.Conn) error {
	var version int
	if err := conn.QueryRow(ctx, `SELECT coalesce(max(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version == 0 {
		return nil
	}
	if err := apply(ctx, conn, version, "down"); err != nil {
		return err
	}
	if version == 1 {
		return nil
	}
	_, err := conn.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version)
	return err
}

func apply(ctx context.Context, conn *pgx.Conn, version int, direction string) error {
	contents, err := migrationFiles.ReadFile("migrations/" + fmt.Sprintf("%06d", version) + "_" + migrationName(version) + "." + direction + ".sql")
	if err != nil {
		return err
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, string(contents)); err != nil {
		return err
	}
	if direction == "up" {
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, version); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func migrationName(version int) string {
	if version == 1 {
		return "foundation"
	}
	if version == 2 {
		return "events"
	}
	if version == 3 {
		return "event_types"
	}
	if version == 4 {
		return "rules"
	}
	if version == 5 {
		return "rule_executions"
	}
	if version == 6 {
		return "rule_versions"
	}
	if version == 7 {
		return "action_execution_attempts"
	}
	if version == 8 {
		return "currencies"
	}
	if version == 9 {
		return "point_ledger"
	}
	if version == 10 {
		return "point_lots"
	}
	if version == 11 {
		return "point_conversions"
	}
	if version == 12 {
		return "progression"
	}
	if version == 13 {
		return "rewards"
	}
	if version == 14 {
		return "achievements"
	}
	if version == 15 {
		return "progression_levels"
	}
	if version == 16 {
		return "rule_schedules"
	}
	if version == 17 {
		return "platform_foundations"
	}
	if version == 18 {
		return "admin_identity"
	}
	if version == 19 {
		return "event_schema_versions"
	}
	if version == 20 {
		return "plan_admin"
	}
	if version == 21 {
		return "reward_reconciliation"
	}
	if version == 22 {
		return "reward_provider_configs"
	}
	if version == 23 {
		return "conversion_lot_backfill"
	}
	if version == 24 {
		return "engagement_extensions"
	}
	if version == 25 {
		return "measurement_foundation"
	}
	if version == 26 {
		return "reports_dashboard"
	}
	return strconv.Itoa(version)
}
