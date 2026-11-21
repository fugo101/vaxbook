// Command migrategen writes a goose migration for the difference between the
// committed migrations and the ent schema (ADR-0009). It uses the Atlas Go library
// through ent, against a throwaway Postgres started with testcontainers, so no
// Atlas CLI or local database is needed.
//
//	go run ./apps/api/internal/store/migrategen -name add_profiles
//
// With -check it writes nothing and exits non-zero if the schema has changes that
// no migration covers (used by CI).
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/sqltool"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/fugo101/vaxbook/apps/api/internal/store"
	"github.com/fugo101/vaxbook/apps/api/internal/store/ent/migrate"
)

const dirPath = "apps/api/internal/store/migrations"

// ent's NamedDiff opens the dev database with the "postgres" driver name.
func init() { sql.Register("postgres", stdlib.GetDefaultDriver()) }

func main() {
	name := flag.String("name", "", "migration name, e.g. add_profiles")
	check := flag.Bool("check", false, "fail if the ent schema has changes not covered by a migration")
	flag.Parse()
	if *name == "" && !*check {
		log.Fatal("usage: migrategen -name <change> | -check")
	}
	if err := run(*name, *check); err != nil {
		log.Fatal(err)
	}
}

func run(name string, check bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ctr, err := postgres.Run(ctx, store.PostgresImage,
		postgres.WithDatabase("migrategen"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return fmt.Errorf("start postgres: %w", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return err
	}

	target := dirPath
	if check {
		// Diff into a scratch copy so the real directory is never touched.
		target, err = copyDir(dirPath)
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(target) }()
		name = "drift_check"
	}

	dir, err := sqltool.NewGooseDir(target)
	if err != nil {
		return err
	}
	// Hand-written migrations are allowed, so recompute atlas.sum first.
	sum, err := dir.Checksum()
	if err != nil {
		return err
	}
	if err := atlasmigrate.WriteSumFile(dir, sum); err != nil {
		return err
	}
	if check {
		if err := sameFile(filepath.Join(dirPath, atlasmigrate.HashFileName), filepath.Join(target, atlasmigrate.HashFileName)); err != nil {
			return err
		}
	}
	wrote, err := diff(ctx, url, dir, target, name, false)
	if err != nil {
		return err
	}
	switch {
	case wrote && check:
		return errors.New("migrategen: the ent schema has changes with no migration; run `task migrate:diff NAME=<change>`")
	case wrote:
		fmt.Println("migrategen: wrote a new migration in", target)
		return nil
	case !check:
		fmt.Println("migrategen: schema and migrations are in sync")
		return nil
	}

	// Generated migrations never drop columns or indexes (expand only, SAD §10), so
	// a schema that removes one diffs as "no change". Catch that narrowing here:
	// it needs a deliberate, hand-written contract migration in a later release.
	narrowDir, err := copyDir(dirPath)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(narrowDir) }()
	nd, err := sqltool.NewGooseDir(narrowDir)
	if err != nil {
		return err
	}
	if sum, err := nd.Checksum(); err != nil {
		return err
	} else if err := atlasmigrate.WriteSumFile(nd, sum); err != nil {
		return err
	}
	if code, _, err := ctr.Exec(ctx, []string{"createdb", "-U", "postgres", "migrategen_narrow"}); err != nil || code != 0 {
		return fmt.Errorf("create scratch database: exit %d: %w", code, err)
	}
	narrowURL := strings.Replace(url, "/migrategen?", "/migrategen_narrow?", 1)
	narrowed, err := diff(ctx, narrowURL, nd, narrowDir, "narrow_check", true)
	if err != nil {
		return err
	}
	if narrowed {
		return errors.New("migrategen: the ent schema drops a column or an index; generated migrations are expand-only, so write the contract migration by hand (SAD §10)")
	}
	fmt.Println("migrategen: schema and migrations are in sync")
	return nil
}

// diff writes a migration for the gap between dir and the ent schema into path
// and reports whether it wrote one. drops allows DROP COLUMN / DROP INDEX.
func diff(ctx context.Context, url string, dir *sqltool.GooseDir, path, name string, drops bool) (bool, error) {
	before, err := countSQL(path)
	if err != nil {
		return false, err
	}
	err = migrate.NamedDiff(ctx, url, name,
		schema.WithDir(dir),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(sqltool.GooseFormatter),
		schema.WithDropColumn(drops),
		schema.WithDropIndex(drops),
	)
	if err != nil && !errors.Is(err, atlasmigrate.ErrNoPlan) {
		return false, fmt.Errorf("diff: %w", err)
	}
	after, err := countSQL(path)
	return after > before, err
}

// countSQL counts the migration files in dir.
func countSQL(dir string) (int, error) {
	m, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	return len(m), err
}

// copyDir copies the migration files (not embed.go) into a new temporary directory.
func copyDir(src string) (string, error) {
	dst, err := os.MkdirTemp("", "migrategen-check-")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == ".go" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o600); err != nil {
			return "", err
		}
	}
	return dst, nil
}

// sameFile fails when the committed atlas.sum is missing or out of date.
func sameFile(committed, fresh string) error {
	a, err := os.ReadFile(committed)
	if err != nil {
		return fmt.Errorf("migrategen: %w; run `task migrate:diff NAME=<change>`", err)
	}
	b, err := os.ReadFile(fresh)
	if err != nil {
		return err
	}
	if string(a) != string(b) {
		return errors.New("migrategen: atlas.sum is out of date; run `task migrate:diff NAME=<change>`")
	}
	return nil
}
