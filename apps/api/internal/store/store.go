// Package store owns the Postgres connection, the ent client and migrations.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/fugo101/vaxbook/apps/api/internal/store/ent"
	"github.com/fugo101/vaxbook/apps/api/internal/store/migrations"
)

// PostgresImage is the Postgres image used by tests and migrategen. Keep it in
// step with the production image (SAD §10).
const PostgresImage = "postgres:19"

// Open connects to Postgres and checks the connection.
func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// NewClient wraps db in an ent client.
func NewClient(db *sql.DB) *ent.Client {
	return ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
}

// Migrate applies every pending migration. A Postgres session advisory lock makes
// it safe when two containers start at once during a deploy (SAD §10).
func Migrate(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	return migrate(ctx, db, migrations.FS, logger)
}

func migrate(ctx context.Context, db *sql.DB, fsys fs.FS, logger *slog.Logger) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("migration locker: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		logger.InfoContext(ctx, "migration applied",
			slog.Int64("version", r.Source.Version),
			slog.Duration("duration", r.Duration))
	}
	return nil
}
