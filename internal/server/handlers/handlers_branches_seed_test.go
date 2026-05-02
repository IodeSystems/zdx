package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type fakeSeeder struct {
	rows  map[string]db.UpsertVersionBranchIfMissingParams
	calls int
	err   error
}

func newFakeSeeder() *fakeSeeder {
	return &fakeSeeder{rows: map[string]db.UpsertVersionBranchIfMissingParams{}}
}

func (f *fakeSeeder) UpsertVersionBranchIfMissing(_ context.Context, arg db.UpsertVersionBranchIfMissingParams) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	if _, exists := f.rows[arg.Name]; exists {
		// Mirror Postgres ON CONFLICT DO NOTHING: row stays as-is.
		return nil
	}
	f.rows[arg.Name] = arg
	return nil
}

func TestSeedClassificationBranches_Library(t *testing.T) {
	q := newFakeSeeder()
	if err := seedClassificationBranches(context.Background(), q, 1, "library"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.rows) != 1 {
		t.Fatalf("expected 1 seed row, got %d: %+v", len(q.rows), q.rows)
	}
	main, ok := q.rows["main"]
	if !ok {
		t.Fatalf("expected 'main' row, got %+v", q.rows)
	}
	if main.Role != "rolling-release" {
		t.Errorf("main.role = %q, want rolling-release", main.Role)
	}
	if main.SourceBranchName.Valid {
		t.Errorf("library main should have no source branch, got %q", main.SourceBranchName.String)
	}
}

func TestSeedClassificationBranches_Service(t *testing.T) {
	q := newFakeSeeder()
	if err := seedClassificationBranches(context.Background(), q, 1, "service"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.rows) != 2 {
		t.Fatalf("expected 2 seed rows, got %d: %+v", len(q.rows), q.rows)
	}
	dev, ok := q.rows["dev"]
	if !ok || dev.Role != "dev" {
		t.Errorf("missing/incorrect dev row: %+v", dev)
	}
	if dev.SourceBranchName.Valid {
		t.Errorf("dev branch should have no source, got %q", dev.SourceBranchName.String)
	}
	main, ok := q.rows["main"]
	if !ok || main.Role != "rolling-release" {
		t.Errorf("missing/incorrect main row: %+v", main)
	}
	if !main.SourceBranchName.Valid || main.SourceBranchName.String != "dev" {
		t.Errorf("service main.source = %+v, want dev", main.SourceBranchName)
	}
}

func TestSeedClassificationBranches_SaaS(t *testing.T) {
	q := newFakeSeeder()
	if err := seedClassificationBranches(context.Background(), q, 1, "saas"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.rows) != 2 {
		t.Fatalf("expected 2 seed rows, got %d: %+v", len(q.rows), q.rows)
	}
	main := q.rows["main"]
	if !main.SourceBranchName.Valid || main.SourceBranchName.String != "dev" {
		t.Errorf("saas main.source = %+v, want dev", main.SourceBranchName)
	}
}

func TestSeedClassificationBranches_ToolAndSite(t *testing.T) {
	for _, class := range []string{"tool", "site"} {
		t.Run(class, func(t *testing.T) {
			q := newFakeSeeder()
			if err := seedClassificationBranches(context.Background(), q, 1, class); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(q.rows) != 1 {
				t.Fatalf("expected 1 seed row, got %d", len(q.rows))
			}
			main := q.rows["main"]
			if main.Role != "rolling-release" {
				t.Errorf("main.role = %q, want rolling-release", main.Role)
			}
		})
	}
}

func TestSeedClassificationBranches_EmptyClassification(t *testing.T) {
	q := newFakeSeeder()
	if err := seedClassificationBranches(context.Background(), q, 1, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.calls != 0 {
		t.Errorf("empty classification should issue zero upserts, got %d", q.calls)
	}
}

func TestSeedClassificationBranches_Idempotent(t *testing.T) {
	q := newFakeSeeder()
	for i := 0; i < 3; i++ {
		if err := seedClassificationBranches(context.Background(), q, 1, "service"); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if len(q.rows) != 2 {
		t.Fatalf("after 3 calls expected 2 rows (idempotent), got %d: %+v", len(q.rows), q.rows)
	}
	// The fake mirrors ON CONFLICT DO NOTHING — the upsert is still issued
	// every time, but the row table only retains the first write.
	if q.calls != 6 {
		t.Errorf("expected 6 upsert calls (3 × 2 rows), got %d", q.calls)
	}
}

// TestSeedClassificationBranches_PreservesManualRow simulates a
// manually-created row (auto_seed=false in real Postgres). The fake's
// ON-CONFLICT-DO-NOTHING behavior covers it: the seed call must not overwrite
// an existing entry under the same (project_id, name) key, so a row with a
// different role / source survives untouched.
func TestSeedClassificationBranches_PreservesManualRow(t *testing.T) {
	q := newFakeSeeder()
	q.rows["main"] = db.UpsertVersionBranchIfMissingParams{
		ProjectID:        1,
		Name:             "main",
		Role:             "named-release",
		SourceBranchName: pgtype.Text{String: "release/v1", Valid: true},
	}
	if err := seedClassificationBranches(context.Background(), q, 1, "service"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	main := q.rows["main"]
	if main.Role != "named-release" {
		t.Errorf("manual main row was overwritten: role = %q, want named-release", main.Role)
	}
	if !main.SourceBranchName.Valid || main.SourceBranchName.String != "release/v1" {
		t.Errorf("manual main row source overwritten: %+v", main.SourceBranchName)
	}
	// The seed run still added the dev row.
	if _, ok := q.rows["dev"]; !ok {
		t.Errorf("expected dev row to be added alongside preserved manual main row")
	}
}

func TestSeedClassificationBranches_PropagatesError(t *testing.T) {
	q := newFakeSeeder()
	q.err = errors.New("boom")
	err := seedClassificationBranches(context.Background(), q, 1, "service")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
}
