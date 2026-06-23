package query

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []token
	}{
		{
			name: "bare word",
			in:   "invoice",
			want: []token{{value: "invoice"}},
		},
		{
			name: "field value",
			in:   "subject:hello",
			want: []token{{field: "subject", hasField: true, value: "hello"}},
		},
		{
			name: "quoted field value",
			in:   `subject:"hi there"`,
			want: []token{{field: "subject", hasField: true, value: "hi there", quoted: true}},
		},
		{
			name: "negated field",
			in:   "-from:bob",
			want: []token{{negated: true, field: "from", hasField: true, value: "bob"}},
		},
		{
			name: "flag without value",
			in:   "unread:",
			want: []token{{field: "unread", hasField: true, value: ""}},
		},
		{
			name: "bare quoted phrase",
			in:   `"quick brown"`,
			want: []token{{value: "quick brown", quoted: true}},
		},
		{
			name: "uppercase field is lowercased",
			in:   "Subject:Hello",
			want: []token{{field: "subject", hasField: true, value: "Hello"}},
		},
		{
			name: "non-identifier colon stays literal",
			in:   "12:30",
			want: []token{{value: "12:30"}},
		},
		{
			name: "multiple terms",
			in:   "  subject:invoice   unread:  bob ",
			want: []token{
				{field: "subject", hasField: true, value: "invoice"},
				{field: "unread", hasField: true, value: ""},
				{value: "bob"},
			},
		},
		{
			name: "negated bare word",
			in:   "-spam",
			want: []token{{negated: true, value: "spam"}},
		},
		{
			name: "email value keeps colon-free address",
			in:   "from:bob@example.com",
			want: []token{{field: "from", hasField: true, value: "bob@example.com"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTokenizeErrors(t *testing.T) {
	_, err := tokenize(`subject:"unterminated`)
	assert.ErrorContains(t, err, "unterminated quote")
}

func TestFTSMatch(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare term", "invoice", `"invoice"`},
		{"subject", "subject:hello", `subject : "hello"`},
		{"sender multi column", "sender:bob", `{from_name from_address} : "bob"`},
		{"recipient", "recipient:alice", `to_text : "alice"`},
		{"body", "body:agenda", `body_text : "agenda"`},
		{"phrase", `subject:"hi there"`, `subject : "hi there"`},
		{"two fields anded", "subject:a body:b", `subject : "a" AND body_text : "b"`},
		{"flags do not appear", "unread: flagged:", ``},
		{"mixed fts and flag", "subject:a unread:", `subject : "a"`},
		{"negation inline", "invoice -reminder", `"invoice" NOT ("reminder")`},
		{"pure negative has no positive expr", "-invoice", ``},
		{"prefix", "subject:inv*", `subject : "inv" *`},
		{"escaped quote", `subject:say"hi`, `subject : "say""hi"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := Parse(tt.in)
			require.NoError(t, err)
			got, err := q.FTSMatch()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"unknown field", "foo:bar", "unknown field"},
		{"empty fts value", `subject:""`, "requires a value"},
		{"invalid bool", "unread:maybe", "invalid boolean value"},
		{"invalid date", "after:soon", "invalid date or duration"},
		{"empty date", "newer:", "requires a date or duration"},
		{"unknown folder", "in:archive", "unknown folder"},
		{"empty folder", "in:", "requires a folder"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.in)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestCompileFolder(t *testing.T) {
	t.Run("folder only", func(t *testing.T) {
		q, err := Parse("in:sent")
		require.NoError(t, err)
		c, err := q.Compile(Options{})
		require.NoError(t, err)
		assert.Equal(t,
			"SELECT m.id, m.date, m.is_read, m.is_flagged, m.has_attachments, m.from_name, m.from_address, m.subject, m.snippet "+
				"FROM messages m WHERE m.folder_id IN (SELECT id FROM folders WHERE type = ?) ORDER BY m.date DESC, m.id DESC",
			c.SQL)
		assert.Equal(t, []any{"sent"}, c.Args)
	})

	t.Run("fts combined with folder", func(t *testing.T) {
		q, err := Parse("subject:invoice in:sent")
		require.NoError(t, err)
		c, err := q.Compile(Options{})
		require.NoError(t, err)
		assert.Contains(t, c.SQL, "WHERE messages_fts MATCH ? AND m.folder_id IN (SELECT id FROM folders WHERE type = ?)")
		assert.Equal(t, []any{`subject : "invoice"`, "sent"}, c.Args)
	})

	t.Run("negated folder", func(t *testing.T) {
		q, err := Parse("-in:trash")
		require.NoError(t, err)
		c, err := q.Compile(Options{})
		require.NoError(t, err)
		assert.Contains(t, c.SQL, "m.folder_id NOT IN (SELECT id FROM folders WHERE type = ?)")
		assert.Equal(t, []any{"trash"}, c.Args)
	})
}

func TestCompileView(t *testing.T) {
	const dedup = "m.rowid IN (SELECT MIN(rowid) FROM messages WHERE message_id IS NOT NULL AND message_id <> '' GROUP BY message_id)"
	const hideSpam = "m.message_id NOT IN (SELECT message_id FROM messages WHERE message_id IS NOT NULL AND message_id <> '' AND folder_id IN (SELECT id FROM folders WHERE type = 'spam'))"

	t.Run("off by default", func(t *testing.T) {
		q, _ := Parse("")
		c, err := q.Compile(Options{})
		require.NoError(t, err)
		assert.NotContains(t, c.SQL, "GROUP BY message_id")
		assert.NotContains(t, c.SQL, "type = 'spam'")
	})

	t.Run("empty query, non-fts shape", func(t *testing.T) {
		q, _ := Parse("")
		c, err := q.Compile(Options{Dedup: true, HideSpam: true})
		require.NoError(t, err)
		assert.Contains(t, c.SQL, dedup)
		assert.Contains(t, c.SQL, hideSpam)
		assert.Empty(t, c.Args, "view predicates carry no bound args")
	})

	t.Run("fts shape carries the predicates too", func(t *testing.T) {
		q, _ := Parse("subject:invoice")
		c, err := q.Compile(Options{Dedup: true, HideSpam: true})
		require.NoError(t, err)
		assert.Contains(t, c.SQL, "messages_fts MATCH ?")
		assert.Contains(t, c.SQL, dedup)
		assert.Contains(t, c.SQL, hideSpam)
		assert.Equal(t, []any{`subject : "invoice"`}, c.Args)
	})

	t.Run("explicit in:spam suppresses HideSpam and dedup", func(t *testing.T) {
		q, _ := Parse("in:spam")
		c, err := q.Compile(Options{Dedup: true, HideSpam: true})
		require.NoError(t, err)
		assert.NotContains(t, c.SQL, "type = 'spam'", "the spam-hiding predicate must be dropped")
		assert.NotContains(t, c.SQL, "GROUP BY message_id", "dedup is dropped for a folder-scoped query")
		assert.Contains(t, c.SQL, "m.folder_id IN (SELECT id FROM folders WHERE type = ?)")
		assert.Equal(t, []any{"spam"}, c.Args)
	})

	t.Run("negated -in:spam keeps HideSpam", func(t *testing.T) {
		q, _ := Parse("-in:spam")
		c, err := q.Compile(Options{Dedup: true, HideSpam: true})
		require.NoError(t, err)
		assert.Contains(t, c.SQL, hideSpam)
	})

	t.Run("a positive folder filter suppresses dedup", func(t *testing.T) {
		q, _ := Parse("in:inbox")
		c, err := q.Compile(Options{Dedup: true, HideSpam: true})
		require.NoError(t, err)
		assert.NotContains(t, c.SQL, "GROUP BY message_id",
			"dedup is unnecessary and harmful when already scoped to one folder")
	})
}

func TestSearchFolderE2E(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	stmts := []string{
		`CREATE TABLE folders (id TEXT PRIMARY KEY, type TEXT)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, folder_id TEXT, subject TEXT,
			from_name TEXT, from_address TEXT, to_text TEXT, snippet TEXT, body_text TEXT,
			date DATETIME NOT NULL, is_read INTEGER, is_flagged INTEGER, has_attachments INTEGER)`,
		`CREATE VIRTUAL TABLE messages_fts USING fts5(subject, from_name, from_address, to_text, snippet, body_text, content=messages, content_rowid=rowid)`,
		`INSERT INTO folders (id, type) VALUES ('fi','inbox'), ('fs','sent')`,
		`INSERT INTO messages (id, folder_id, subject, snippet, body_text, date, is_read, is_flagged, has_attachments) VALUES
			('a','fi','Inbox hello','hi','body one','2024-01-01',0,0,0),
			('b','fs','Sent reply','re','body two','2024-02-01',1,0,0),
			('c','fs','Sent invoice','inv','body three','2024-03-01',1,0,0)`,
		`INSERT INTO messages_fts(rowid, subject, from_name, from_address, to_text, snippet, body_text)
			SELECT rowid, subject, from_name, from_address, '', snippet, body_text FROM messages`,
	}
	for _, s := range stmts {
		_, err := db.Exec(s)
		require.NoError(t, err)
	}

	run := func(qs string) []string {
		t.Helper()
		q, err := Parse(qs)
		require.NoError(t, err)
		c, err := q.Compile(Options{})
		require.NoError(t, err)
		rows, err := db.Query(c.SQL, c.Args...)
		require.NoError(t, err, "exec %q -> %s", qs, c.SQL)
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id, date, su, sn string
			var fn, fa sql.NullString
			var r, f, a int
			require.NoError(t, rows.Scan(&id, &date, &r, &f, &a, &fn, &fa, &su, &sn))
			ids = append(ids, id)
		}
		return ids
	}

	assert.ElementsMatch(t, []string{"b", "c"}, run("in:sent"))
	assert.ElementsMatch(t, []string{"a"}, run("in:inbox"))
	assert.ElementsMatch(t, []string{"a"}, run("-in:sent"))
	assert.ElementsMatch(t, []string{"c"}, run("in:sent subject:invoice"))
}

func TestParseRelSpec(t *testing.T) {
	tests := []struct {
		in   string
		want RelSpec
	}{
		{"7d", RelSpec{Days: 7}},
		{"7days", RelSpec{Days: 7}},
		{"1w", RelSpec{Days: 7}},
		{"2weeks", RelSpec{Days: 14}},
		{"1m", RelSpec{Months: 1}},
		{"1mo", RelSpec{Months: 1}},
		{"3month", RelSpec{Months: 3}},
		{"1y", RelSpec{Years: 1}},
		{"2years", RelSpec{Years: 2}},
		{"24h", RelSpec{Dur: 24 * time.Hour}},
		{"2 months", RelSpec{Months: 2}}, // optional space
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := parseRelSpec(tt.in)
			require.True(t, ok)
			assert.Equal(t, tt.want, *got)
		})
	}

	for _, bad := range []string{"", "abc", "d", "7", "7lightyears"} {
		t.Run("invalid/"+bad, func(t *testing.T) {
			_, ok := parseRelSpec(bad)
			assert.False(t, ok)
		})
	}
}

func TestParseWhenAbsolute(t *testing.T) {
	at, rel, err := parseWhen("2024-01-15")
	require.NoError(t, err)
	assert.Nil(t, rel)
	assert.Equal(t, time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local), at)
}

func TestCompileDate(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	t.Run("relative newer", func(t *testing.T) {
		q, err := Parse("newer:1month")
		require.NoError(t, err)
		c, err := q.Compile(Options{Now: now})
		require.NoError(t, err)
		assert.Equal(t,
			"SELECT m.id, m.date, m.is_read, m.is_flagged, m.has_attachments, m.from_name, m.from_address, m.subject, m.snippet "+
				"FROM messages m WHERE julianday(m.date) >= julianday(?) ORDER BY m.date DESC, m.id DESC",
			c.SQL)
		assert.Equal(t, []any{"2024-02-15 12:00:00"}, c.Args)
	})

	t.Run("absolute before", func(t *testing.T) {
		q, err := Parse("before:2024-02-01")
		require.NoError(t, err)
		c, err := q.Compile(Options{Now: now})
		require.NoError(t, err)
		assert.Contains(t, c.SQL, "julianday(m.date) < julianday(?)")
		assert.Equal(t, []any{time.Date(2024, 2, 1, 0, 0, 0, 0, time.Local).UTC().Format("2006-01-02 15:04:05")}, c.Args)
	})

	t.Run("negated newer flips to older", func(t *testing.T) {
		q, err := Parse("-newer:7d")
		require.NoError(t, err)
		c, err := q.Compile(Options{Now: now})
		require.NoError(t, err)
		assert.Contains(t, c.SQL, "julianday(m.date) < julianday(?)")
		assert.Equal(t, []any{"2024-03-08 12:00:00"}, c.Args)
	})

	t.Run("fts combined with date", func(t *testing.T) {
		q, err := Parse("subject:invoice newer:2months")
		require.NoError(t, err)
		c, err := q.Compile(Options{Now: now})
		require.NoError(t, err)
		assert.Equal(t,
			"SELECT m.id, m.date, m.is_read, m.is_flagged, m.has_attachments, m.from_name, m.from_address, m.subject, m.snippet "+
				"FROM messages_fts JOIN messages m ON m.rowid = messages_fts.rowid "+
				"WHERE messages_fts MATCH ? AND julianday(m.date) >= julianday(?) ORDER BY rank, m.id",
			c.SQL)
		assert.Equal(t, []any{`subject : "invoice"`, "2024-01-15 12:00:00"}, c.Args)
	})
}

func TestCompileSQL(t *testing.T) {
	t.Run("positive fts with flag", func(t *testing.T) {
		q, err := Parse("subject:invoice unread:")
		require.NoError(t, err)
		c, err := q.Compile(Options{Limit: 50})
		require.NoError(t, err)
		assert.Equal(t,
			"SELECT m.id, m.date, m.is_read, m.is_flagged, m.has_attachments, m.from_name, m.from_address, m.subject, m.snippet "+
				"FROM messages_fts JOIN messages m ON m.rowid = messages_fts.rowid "+
				"WHERE messages_fts MATCH ? AND m.is_read = ? ORDER BY rank, m.id LIMIT ?",
			c.SQL)
		assert.Equal(t, []any{`subject : "invoice"`, 0, 50}, c.Args)
	})

	t.Run("flag only", func(t *testing.T) {
		q, err := Parse("flagged:")
		require.NoError(t, err)
		c, err := q.Compile(Options{})
		require.NoError(t, err)
		assert.Equal(t,
			"SELECT m.id, m.date, m.is_read, m.is_flagged, m.has_attachments, m.from_name, m.from_address, m.subject, m.snippet "+
				"FROM messages m WHERE m.is_flagged = ? ORDER BY m.date DESC, m.id DESC",
			c.SQL)
		assert.Equal(t, []any{1}, c.Args)
	})

	t.Run("pure negative uses NOT IN subquery", func(t *testing.T) {
		q, err := Parse("-invoice")
		require.NoError(t, err)
		c, err := q.Compile(Options{})
		require.NoError(t, err)
		assert.Equal(t,
			"SELECT m.id, m.date, m.is_read, m.is_flagged, m.has_attachments, m.from_name, m.from_address, m.subject, m.snippet "+
				"FROM messages m WHERE m.rowid NOT IN (SELECT rowid FROM messages_fts WHERE messages_fts MATCH ?) ORDER BY m.date DESC, m.id DESC",
			c.SQL)
		assert.Equal(t, []any{`"invoice"`}, c.Args)
	})

	t.Run("empty query matches all", func(t *testing.T) {
		q, err := Parse("   ")
		require.NoError(t, err)
		c, err := q.Compile(Options{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t,
			"SELECT m.id, m.date, m.is_read, m.is_flagged, m.has_attachments, m.from_name, m.from_address, m.subject, m.snippet "+
				"FROM messages m ORDER BY m.date DESC, m.id DESC LIMIT ?",
			c.SQL)
		assert.Equal(t, []any{10}, c.Args)
	})

	t.Run("negated flag flips value", func(t *testing.T) {
		q, err := Parse("-unread:")
		require.NoError(t, err)
		c, err := q.Compile(Options{})
		require.NoError(t, err)
		assert.Contains(t, c.SQL, "m.is_read = ?")
		assert.Equal(t, []any{1}, c.Args)
	})
}

// --- End-to-end against a real FTS5 database -------------------------------

type testMsg struct {
	id                                                 string
	subject, fromName, fromAddr, toText, snippet, body string
	isRead, isFlagged, hasAttach                       int
	date                                               string
}

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	schema := []string{
		`CREATE TABLE messages (
			id TEXT PRIMARY KEY, subject TEXT, from_name TEXT, from_address TEXT,
			to_text TEXT, snippet TEXT, body_text TEXT,
			is_read INTEGER DEFAULT 0, is_flagged INTEGER DEFAULT 0,
			has_attachments INTEGER DEFAULT 0, date DATETIME NOT NULL)`,
		`CREATE VIRTUAL TABLE messages_fts USING fts5(
			subject, from_name, from_address, to_text, snippet, body_text,
			content=messages, content_rowid=rowid)`,
	}
	for _, s := range schema {
		_, err := db.Exec(s)
		require.NoError(t, err)
	}

	msgs := []testMsg{
		{"1", "Invoice 2024", "Bob Smith", "bob@example.com", "Alice alice@example.com", "your invoice is ready", "please find the invoice attached", 0, 1, 1, "2024-01-01"},
		{"2", "Meeting notes", "Carol Jones", "carol@work.com", "team team@work.com", "notes from the meeting", "agenda and action items", 1, 0, 0, "2024-02-01"},
		{"3", "Invoice reminder", "Bob Smith", "bob@example.com", "Alice alice@example.com", "your reminder about the invoice", "second notice", 1, 0, 0, "2024-03-01"},
	}
	for _, m := range msgs {
		_, err := db.Exec(
			`INSERT INTO messages (id, subject, from_name, from_address, to_text, snippet, body_text, is_read, is_flagged, has_attachments, date)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			m.id, m.subject, m.fromName, m.fromAddr, m.toText, m.snippet, m.body, m.isRead, m.isFlagged, m.hasAttach, m.date)
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT INTO messages_fts(rowid, subject, from_name, from_address, to_text, snippet, body_text)
		SELECT rowid, subject, from_name, from_address, to_text, snippet, body_text FROM messages`)
	require.NoError(t, err)

	return db
}

func runIDs(t *testing.T, db *sql.DB, queryStr string) []string {
	t.Helper()
	return runIDsOpts(t, db, queryStr, Options{})
}

func runIDsOpts(t *testing.T, db *sql.DB, queryStr string, opts Options) []string {
	t.Helper()
	q, err := Parse(queryStr)
	require.NoError(t, err)
	c, err := q.Compile(opts)
	require.NoError(t, err, "compile %q", queryStr)

	rows, err := db.Query(c.SQL, c.Args...)
	require.NoError(t, err, "exec %q -> %s", queryStr, c.SQL)
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id, date, fromName, fromAddr, subject, snippet string
		var isRead, isFlagged, hasAttach int
		require.NoError(t, rows.Scan(&id, &date, &isRead, &isFlagged, &hasAttach, &fromName, &fromAddr, &subject, &snippet))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	return ids
}

func TestSearchE2E(t *testing.T) {
	db := setupDB(t)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"subject filter", "subject:invoice", []string{"1", "3"}},
		{"subject + unread", "subject:invoice unread:", []string{"1"}},
		{"sender name", "sender:bob", []string{"1", "3"}},
		{"sender address", "from:carol", []string{"2"}},
		{"recipient", "recipient:team", []string{"2"}},
		{"body", "body:agenda", []string{"2"}},
		{"bare term across columns", "invoice", []string{"1", "3"}},
		{"flagged only", "flagged:", []string{"1"}},
		{"attachments only", "attachments:", []string{"1"}},
		{"read flag", "read:", []string{"2", "3"}},
		{"pure negative", "-invoice", []string{"2"}},
		{"positive minus negative", "invoice -reminder", []string{"1"}},
		{"prefix match", "subject:inv*", []string{"1", "3"}},
		{"phrase", `snippet:"your invoice"`, []string{"1"}},
		{"empty matches all", "", []string{"1", "2", "3"}},
		{"no results", "subject:doesnotexist", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runIDs(t, db, tt.query)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestSearchDateE2E(t *testing.T) {
	db := setupDB(t)
	// Messages are dated 2024-01-01 (m1), 2024-02-01 (m2), 2024-03-01 (m3).
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"after absolute", "after:2024-02-15", []string{"3"}},
		{"before absolute", "before:2024-02-15", []string{"1", "2"}},
		{"since includes all", "since:2024-01-01", []string{"1", "2", "3"}},
		{"newer 1month", "newer:1month", []string{"3"}},
		{"newer 15d", "newer:15d", []string{"3"}},
		{"older 2months", "older:2months", []string{"1"}},
		{"fts and date", "subject:invoice older:2months", []string{"1"}},
		{"negated newer", "-newer:2months", []string{"1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runIDsOpts(t, db, tt.query, Options{Now: now})
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
