// Package storetest gives tests a real, migrated Postgres.
//
// One container is started per test binary (in TestMain). Migrations run once
// into a template database; each test then gets its own database cloned from the
// template, so tests are isolated and may run in parallel.
//
// Docker must be reachable. With Docker contexts other than the default socket
// (OrbStack, Colima…), set DOCKER_HOST; `task test` does this for you.
package storetest

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/fugo101/vaxbook/apps/api/internal/store"
)

const templateDB = "vaxbook_template"

// Server is a running Postgres container with a migrated template database.
type Server struct {
	ctr     *postgres.PostgresContainer
	baseURL *url.URL // points at the maintenance database "postgres"
	admin   *sql.DB
	seq     atomic.Int64
}

// Main starts the server, runs the tests and tears the server down. Use it as:
//
//	var pg *storetest.Server
//	func TestMain(m *testing.M) { os.Exit(storetest.Main(m, &pg)) }
func Main(m *testing.M, srv **Server) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	s, err := start(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "storetest:", err)
		return 1
	}
	*srv = s
	code := m.Run()
	s.close()
	return code
}

func start(ctx context.Context) (*Server, error) {
	ctr, err := postgres.Run(ctx, store.PostgresImage,
		postgres.WithDatabase("postgres"),
		postgres.BasicWaitStrategies(),
		// Durability is pointless for throwaway test data; this makes tests faster.
		testcontainers.WithCmd("postgres", "-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off"),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	s := &Server{ctr: ctr}
	if err := s.init(ctx); err != nil {
		s.close()
		return nil, err
	}
	return s, nil
}

func (s *Server) init(ctx context.Context) error {
	raw, err := s.ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return err
	}
	if s.baseURL, err = url.Parse(raw); err != nil {
		return err
	}
	if s.admin, err = store.Open(ctx, raw); err != nil {
		return err
	}
	if _, err := s.admin.ExecContext(ctx, "CREATE DATABASE "+templateDB); err != nil {
		return fmt.Errorf("create template: %w", err)
	}
	tdb, err := store.Open(ctx, s.urlFor(templateDB))
	if err != nil {
		return err
	}
	defer func() { _ = tdb.Close() }() // a template must have no open connections when cloned
	if err := store.Migrate(ctx, tdb, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		return fmt.Errorf("migrate template: %w", err)
	}
	return nil
}

// NewDB returns a fresh migrated database for t, dropped when t ends.
func (s *Server) NewDB(t testing.TB) *sql.DB {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("test_%d", s.seq.Add(1))
	if _, err := s.admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateDB)); err != nil {
		t.Fatalf("storetest: create database: %v", err)
	}
	db, err := store.Open(ctx, s.urlFor(name))
	if err != nil {
		t.Fatalf("storetest: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = s.admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	})
	return db
}

func (s *Server) urlFor(db string) string {
	u := *s.baseURL
	u.Path = "/" + db
	return u.String()
}

func (s *Server) close() {
	if s.admin != nil {
		_ = s.admin.Close()
	}
	if s.ctr != nil {
		_ = testcontainers.TerminateContainer(s.ctr)
	}
}
