package server

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SchemaFeatures records which optional schema additions are present.
// Detected once at startup via information_schema inspection.
//
// Migration rules for straddle-safe deploys:
//  - Add columns as NOT NULL DEFAULT <val> (or nullable) so old binaries
//    that don't SELECT them continue to work against the new schema.
//  - Never remove or rename columns in a single deploy. Deprecate first
//    (stop writing), deploy, then drop in a follow-up migration.
//  - Never change column types incompatibly.
//  - Handlers that use new schema additions must gate on the corresponding
//    SchemaFeature flag and return 503 gracefully if absent.
//  - compat-check will flag incompatible migrations; use
//    bin/ship --non-compatible-migration for breaking changes.
type SchemaFeatures struct {
	// HasLLMConfig is true when the zdx_llm_configs table exists (migration 018).
	HasLLMConfig bool
	// HasProjectGitConfig is true when zdx_projects has git_url (migration 019).
	HasProjectGitConfig bool
}

// detectFeatures inspects information_schema to determine which optional
// schema additions are present. Safe to call before the sqlc queries are
// used, since it only reads system catalog tables.
func detectFeatures(ctx context.Context, pool *pgxpool.Pool) SchemaFeatures {
	var f SchemaFeatures

	tableExists := func(table string) bool {
		var n int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.tables
			 WHERE table_schema = 'public' AND table_name = $1`, table).Scan(&n)
		return err == nil && n > 0
	}

	columnExists := func(table, column string) bool {
		var n int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.columns
			 WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`,
			table, column).Scan(&n)
		return err == nil && n > 0
	}

	f.HasLLMConfig = tableExists("zdx_llm_configs")
	f.HasProjectGitConfig = columnExists("zdx_projects", "git_url")

	log.Printf("schema features: llm_config=%v project_git_config=%v",
		f.HasLLMConfig, f.HasProjectGitConfig)
	return f
}
