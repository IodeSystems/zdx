package e2e

// TestEventStreamBackfill is the integration test for migration 122
// (122_event_stream_backfill.up.sql, IS-614 / TK-1378).
//
// It provisions a fresh database, migrates to version 121 (events schema
// without the backfill), seeds legacy tables, runs migration 122, then
// asserts:
//   - 3 comment events (1 root + 2 replies)
//   - 1 thread with root_event_id pointing at the root comment's event
//   - replies have thread_id set; root event has thread_id IS NULL
//   - 2 revision events
//   - all 5 backfilled events have agent_process_result IS NULL
//   - zdx_comment_reactions and zdx_comment_reads tables are gone
//   - zdx_comments and zdx_proposal_versions still exist with seeded rows

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iodesystems/zdx-go/internal/migrate"
)

func TestEventStreamBackfill(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable (external runner mode); needs direct DB access")
	}

	ctx := context.Background()

	// Provision a fresh isolated DB so we can start from a known migration state.
	testDSN, dropDB := createMigrationTestDB(t, srv.DSN)
	defer dropDB()

	// Migrate up to 121 (zdx_events/zdx_event_threads tables exist, backfill not yet applied).
	migrateDSN := strings.Replace(testDSN, "postgres://", "pgx5://", 1)
	if err := migrate.MigrateTo(migrateDSN, 121); err != nil {
		t.Fatalf("migrate to 121: %v", err)
	}

	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	// Bootstrap: insert a project so FK constraints on comments/proposals are satisfied.
	var projectID int32
	if err := conn.QueryRow(ctx, `
		INSERT INTO zdx_projects (slug, name, classification)
		VALUES ('backfill-test', 'Backfill Test', 'service')
		RETURNING id
	`).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	// Seed legacy data ----------------------------------------------------------

	// 1 root comment.
	var rootCommentID int32
	if err := conn.QueryRow(ctx, `
		INSERT INTO zdx_comments (project_id, target_type, target_id, author, body, author_alias)
		VALUES ($1, 'proposal', 'PR-test', 'alice', 'root comment', '')
		RETURNING id
	`, projectID).Scan(&rootCommentID); err != nil {
		t.Fatalf("insert root comment: %v", err)
	}

	// 1 direct reply to root.
	var reply1ID int32
	if err := conn.QueryRow(ctx, `
		INSERT INTO zdx_comments (project_id, target_type, target_id, author, body, parent_id, author_alias)
		VALUES ($1, 'proposal', 'PR-test', 'bob', 'reply 1', $2, '')
		RETURNING id
	`, projectID, rootCommentID).Scan(&reply1ID); err != nil {
		t.Fatalf("insert reply 1: %v", err)
	}

	// 1 nested reply (reply to reply) — agent-authored.
	var reply2ID int32
	if err := conn.QueryRow(ctx, `
		INSERT INTO zdx_comments (project_id, target_type, target_id, author, body, parent_id, author_alias)
		VALUES ($1, 'proposal', 'PR-test', 'agent-x', 'reply 2', $2, 'agent-alias')
		RETURNING id
	`, projectID, reply1ID).Scan(&reply2ID); err != nil {
		t.Fatalf("insert reply 2: %v", err)
	}

	// 1 reaction and 1 read (only to verify the tables are dropped by the migration).
	if _, err := conn.Exec(ctx, `
		INSERT INTO zdx_comment_reactions (project_id, comment_id, emoji, reactor)
		VALUES ($1, $2, '👍', 'alice')
	`, projectID, rootCommentID); err != nil {
		t.Fatalf("insert reaction: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO zdx_comment_reads (project_id, target_type, target_id, role)
		VALUES ($1, 'proposal', 'PR-test', 'user')
	`, projectID); err != nil {
		t.Fatalf("insert comment_read: %v", err)
	}

	// 1 proposal + 2 versions.
	var proposalID int32
	if err := conn.QueryRow(ctx, `
		INSERT INTO zdx_proposals (project_id, title, created_by)
		VALUES ($1, 'Test Proposal', 'alice') RETURNING id
	`, projectID).Scan(&proposalID); err != nil {
		t.Fatalf("insert proposal: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO zdx_proposal_versions (proposal_id, body, edited_by, edited_at)
		VALUES ($1, 'body v1', 'alice', $2)
	`, proposalID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("insert version 1: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO zdx_proposal_versions (proposal_id, body, edited_by, edited_at)
		VALUES ($1, 'body v2', 'bob', $2)
	`, proposalID, time.Now()); err != nil {
		t.Fatalf("insert version 2: %v", err)
	}

	// Apply migration 122 (the backfill) ----------------------------------------
	if err := migrate.MigrateTo(migrateDSN, 122); err != nil {
		t.Fatalf("migrate to 122: %v", err)
	}

	// Assertions ----------------------------------------------------------------

	// Reconnect after migration (connection may be stale after DDL).
	conn.Close(ctx)
	conn, err = pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	defer conn.Close(ctx)

	// 3 comment events.
	var commentCount int
	conn.QueryRow(ctx, `SELECT count(*) FROM zdx_events WHERE event_type='comment'`).Scan(&commentCount) //nolint:errcheck
	if commentCount != 3 {
		t.Errorf("comment events: got %d, want 3", commentCount)
	}

	// 1 thread.
	var threadCount int
	conn.QueryRow(ctx, `SELECT count(*) FROM zdx_event_threads`).Scan(&threadCount) //nolint:errcheck
	if threadCount != 1 {
		t.Errorf("threads: got %d, want 1", threadCount)
	}

	// Thread's root_event_id matches the root comment's event.
	var threadID, rootEventID int64
	conn.QueryRow(ctx, `SELECT id, root_event_id FROM zdx_event_threads LIMIT 1`).Scan(&threadID, &rootEventID) //nolint:errcheck

	var rootEventLegacyID int
	if err := conn.QueryRow(ctx, `
		SELECT (detail_json->>'legacy_comment_id')::int
		FROM zdx_events WHERE id = $1
	`, rootEventID).Scan(&rootEventLegacyID); err != nil {
		t.Fatalf("lookup root event: %v", err)
	}
	if rootEventLegacyID != int(rootCommentID) {
		t.Errorf("root_event_id points to legacy_comment_id=%d, want %d", rootEventLegacyID, rootCommentID)
	}

	// Root comment event: thread_id IS NULL.
	var rootThreadIDNull bool
	conn.QueryRow(ctx, `
		SELECT thread_id IS NULL FROM zdx_events
		WHERE (detail_json->>'legacy_comment_id')::int = $1
	`, rootCommentID).Scan(&rootThreadIDNull) //nolint:errcheck
	if !rootThreadIDNull {
		t.Error("root comment event should have thread_id IS NULL")
	}

	// Both reply events have thread_id = threadID.
	var replyThreadIDs []int64
	rows, _ := conn.Query(ctx, `
		SELECT thread_id FROM zdx_events
		WHERE event_type='comment'
		  AND thread_id IS NOT NULL
		ORDER BY id
	`)
	defer rows.Close()
	for rows.Next() {
		var tid int64
		rows.Scan(&tid) //nolint:errcheck
		replyThreadIDs = append(replyThreadIDs, tid)
	}
	if len(replyThreadIDs) != 2 {
		t.Errorf("reply events with thread_id: got %d, want 2", len(replyThreadIDs))
	}
	for _, tid := range replyThreadIDs {
		if tid != threadID {
			t.Errorf("reply thread_id=%d, want %d", tid, threadID)
		}
	}

	// 2 revision events.
	var revisionCount int
	conn.QueryRow(ctx, `SELECT count(*) FROM zdx_events WHERE event_type='revision'`).Scan(&revisionCount) //nolint:errcheck
	if revisionCount != 2 {
		t.Errorf("revision events: got %d, want 2", revisionCount)
	}

	// All 5 backfilled events have agent_process_result IS NULL.
	var nonNullAPR int
	conn.QueryRow(ctx, `SELECT count(*) FROM zdx_events WHERE agent_process_result IS NOT NULL`).Scan(&nonNullAPR) //nolint:errcheck
	if nonNullAPR != 0 {
		t.Errorf("events with non-NULL agent_process_result: got %d, want 0", nonNullAPR)
	}

	// zdx_comment_reactions dropped.
	var reactionsOID *string
	conn.QueryRow(ctx, `SELECT to_regclass('zdx_comment_reactions')::text`).Scan(&reactionsOID) //nolint:errcheck
	if reactionsOID != nil {
		t.Errorf("zdx_comment_reactions still exists: %s", *reactionsOID)
	}

	// zdx_comment_reads dropped.
	var readsOID *string
	conn.QueryRow(ctx, `SELECT to_regclass('zdx_comment_reads')::text`).Scan(&readsOID) //nolint:errcheck
	if readsOID != nil {
		t.Errorf("zdx_comment_reads still exists: %s", *readsOID)
	}

	// zdx_comments still exists with seeded rows.
	var commentRowCount int
	conn.QueryRow(ctx, `SELECT count(*) FROM zdx_comments`).Scan(&commentRowCount) //nolint:errcheck
	if commentRowCount != 3 {
		t.Errorf("zdx_comments rows: got %d, want 3", commentRowCount)
	}

	// zdx_proposal_versions still exists with seeded rows.
	var versionRowCount int
	conn.QueryRow(ctx, `SELECT count(*) FROM zdx_proposal_versions`).Scan(&versionRowCount) //nolint:errcheck
	if versionRowCount != 2 {
		t.Errorf("zdx_proposal_versions rows: got %d, want 2", versionRowCount)
	}

	// Events ordered by created_at ASC: root first, then replies by insertion order.
	var orderedLegacyIDs []int
	rows2, _ := conn.Query(ctx, `
		SELECT (detail_json->>'legacy_comment_id')::int
		FROM zdx_events
		WHERE event_type='comment'
		ORDER BY created_at ASC, id ASC
	`)
	defer rows2.Close()
	for rows2.Next() {
		var lid int
		rows2.Scan(&lid) //nolint:errcheck
		orderedLegacyIDs = append(orderedLegacyIDs, lid)
	}
	want := []int{int(rootCommentID), int(reply1ID), int(reply2ID)}
	for i, got := range orderedLegacyIDs {
		if got != want[i] {
			t.Errorf("comment event order[%d]: got legacy_id=%d, want %d", i, got, want[i])
		}
	}
}

// createMigrationTestDB creates a fresh database in the same postgres cluster
// as baseDSN, returning a DSN for it and a cleanup func that drops the DB.
// The zdx user is the cluster superuser in the e2e compose setup.
func createMigrationTestDB(t *testing.T, baseDSN string) (string, func()) {
	t.Helper()
	ctx := context.Background()

	u, err := url.Parse(baseDSN)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	adminDSN := *u
	adminDSN.Path = "/postgres"
	dbName := fmt.Sprintf("zdx_migtest_%d", time.Now().UnixNano())

	adminConn, err := pgx.Connect(ctx, adminDSN.String())
	if err != nil {
		t.Fatalf("connect to postgres db: %v", err)
	}
	defer adminConn.Close(ctx)

	if _, err := adminConn.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("create database %s: %v", dbName, err)
	}

	newDSN := *u
	newDSN.Path = "/" + dbName

	drop := func() {
		conn, err := pgx.Connect(context.Background(), adminDSN.String())
		if err != nil {
			return
		}
		defer conn.Close(context.Background())
		conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName) //nolint:errcheck
	}
	return newDSN.String(), drop
}
