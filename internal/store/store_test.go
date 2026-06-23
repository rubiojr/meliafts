package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "melia.db")

	w, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer w.Close()

	stmts := []string{
		`CREATE TABLE messages (
			id TEXT PRIMARY KEY, subject TEXT, from_name TEXT, from_address TEXT,
			to_addresses TEXT, snippet TEXT, body_text TEXT, body_html TEXT,
			date DATETIME NOT NULL, is_read INTEGER DEFAULT 0, is_flagged INTEGER DEFAULT 0,
			has_attachments INTEGER DEFAULT 0)`,
		`CREATE VIRTUAL TABLE messages_fts USING fts5(
			subject, from_name, from_address, to_text, snippet, body_text,
			content=messages, content_rowid=rowid)`,
		`INSERT INTO messages (id, subject, from_name, from_address, to_addresses, snippet, body_text, date, is_read, is_flagged, has_attachments) VALUES
			('m1','Invoice 2024','Bob Smith','bob@acme.com','[{"address":"alice@acme.com"}]','your invoice','Full invoice body here.','2024-01-15 09:30:00',0,1,1),
			('m2','Meeting notes','Carol','carol@work.com','[]','standup notes','Agenda and items.','2024-02-20 14:00:00',1,0,0)`,
		`INSERT INTO messages_fts(rowid, subject, from_name, from_address, to_text, snippet, body_text)
			SELECT rowid, subject, from_name, from_address, '', snippet, body_text FROM messages`,
	}
	for _, s := range stmts {
		_, err := w.Exec(s)
		require.NoError(t, err)
	}
	return path
}

func TestStoreSearch(t *testing.T) {
	st, err := Open(newTestDB(t))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	got, err := st.Search("subject:invoice", 50)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "m1", got[0].ID)
	assert.Equal(t, "Invoice 2024", got[0].Subject)
	assert.False(t, got[0].IsRead)
	assert.True(t, got[0].IsFlagged)
	// Search populates list fields only.
	assert.Empty(t, got[0].BodyText)

	all, err := st.Search("", 50)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestStoreSearchInvalidQuery(t *testing.T) {
	st, err := Open(newTestDB(t))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	_, err = st.Search("bogus:field", 50)
	assert.ErrorContains(t, err, "unknown field")
}

func TestStoreLoad(t *testing.T) {
	st, err := Open(newTestDB(t))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	m, err := st.Load("m1")
	require.NoError(t, err)
	assert.Equal(t, "Invoice 2024", m.Subject)
	assert.Equal(t, "Full invoice body here.", m.BodyText)
	assert.Contains(t, m.ToAddresses, "alice@acme.com")

	_, err = st.Load("nope")
	assert.Error(t, err)
}
