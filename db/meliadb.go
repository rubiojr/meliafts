// Package meliadb embeds the melia database schema and applies it to a fresh
// SQLite database. It is used to build sample/fixture databases.
package meliadb

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

// schemaFile is the exact schema dumped from a real melia database.
//
//go:embed schema/v1.1.242.sql
var schemaFile string

// Schema returns the melia schema with the statements that SQLite creates
// automatically removed, so it can be applied to a fresh database. A raw
// `.schema` dump re-declares sqlite_sequence (created by AUTOINCREMENT) and the
// FTS5 shadow tables (created by the messages_fts virtual table), which would
// fail with "table already exists".
func Schema() string {
	var b strings.Builder
	for line := range strings.SplitSeq(schemaFile, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "CREATE TABLE sqlite_sequence") ||
			strings.HasPrefix(t, "CREATE TABLE 'messages_fts_") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// Apply creates all melia tables, indexes and triggers on db. The triggers keep
// messages_fts and folders.unread_count in sync, so callers only need to insert
// into messages afterwards.
func Apply(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, Schema()); err != nil {
		return fmt.Errorf("apply melia schema: %w", err)
	}
	return nil
}
