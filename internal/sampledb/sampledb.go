// Package sampledb builds a sample melia database populated with random but
// realistic-looking emails. It is used both as a demo database and as a
// deterministic fixture for end-to-end tests.
package sampledb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	meliadb "github.com/rubiojr/meliafts/db"
)

const (
	accountID    = "acct-demo"
	accountEmail = "you@example.com"
	accountName  = "Demo User"
)

// Options controls generation.
type Options struct {
	// Seed makes generation deterministic. Defaults to 1.
	Seed int64
	// Messages is the number of random messages to generate (in addition to a
	// handful of curated demo messages). Defaults to 140.
	Messages int
	// Now is the reference time message dates are spread back from. Defaults to
	// time.Now(). Pass a fixed value for reproducible fixtures.
	Now time.Time
}

func (o Options) withDefaults() Options {
	if o.Seed == 0 {
		o.Seed = 1
	}
	if o.Messages == 0 {
		o.Messages = 140
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	return o
}

type addr struct{ Name, Email string }

// Build creates a fresh SQLite database at path and fills it with sample data.
// An existing file is replaced.
func Build(ctx context.Context, path string, opts Options) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := Generate(ctx, db, opts); err != nil {
		return err
	}
	return db.Close()
}

// Generate applies the melia schema to db and inserts the sample account,
// folders and messages. The schema's triggers keep messages_fts and the folder
// counters in sync automatically.
func Generate(ctx context.Context, db *sql.DB, opts Options) error {
	opts = opts.withDefaults()

	if err := meliadb.Apply(ctx, db); err != nil {
		return err
	}
	if err := insertAccount(ctx, db); err != nil {
		return err
	}
	if err := insertFolders(ctx, db); err != nil {
		return err
	}
	return insertMessages(ctx, db, opts)
}

func insertAccount(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `INSERT INTO accounts
		(id,name,email,type,incoming_host,incoming_port,incoming_security,
		 outgoing_host,outgoing_port,outgoing_security,username,account_is_default)
		VALUES (?,?,?,'imap','imap.example.com',993,'ssl','smtp.example.com',465,'ssl',?,1)`,
		accountID, accountName, accountEmail, accountEmail)
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

// folders maps a folder id to its (name, path, type).
var folders = []struct{ id, name, path, ftype string }{
	{"f-inbox", "Inbox", "INBOX", "inbox"},
	{"f-sent", "Sent", "Sent", "sent"},
	{"f-drafts", "Drafts", "Drafts", "drafts"},
	{"f-spam", "Spam", "Spam", "spam"},
	{"f-trash", "Trash", "Trash", "trash"},
}

func insertFolders(ctx context.Context, db *sql.DB) error {
	for _, f := range folders {
		_, err := db.ExecContext(ctx,
			`INSERT INTO folders (id,account_id,name,path,type) VALUES (?,?,?,?,?)`,
			f.id, accountID, f.name, f.path, f.ftype)
		if err != nil {
			return fmt.Errorf("insert folder %s: %w", f.id, err)
		}
	}
	return nil
}

func insertMessages(ctx context.Context, db *sql.DB, opts Options) error {
	rng := rand.New(rand.NewSource(opts.Seed))

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	msgs := curatedMessages(opts.Now)
	for i := 0; i < opts.Messages; i++ {
		msgs = append(msgs, randomMessage(rng, opts.Now))
	}

	for i, m := range msgs {
		m.id = fmt.Sprintf("msg-%05d", i+1)
		if err := m.insert(ctx, stmt); err != nil {
			return fmt.Errorf("insert %s: %w", m.id, err)
		}
	}
	return tx.Commit()
}

const insertSQL = `INSERT INTO messages
	(id, account_id, folder_id, message_id, thread_id,
	 from_address, from_name, to_addresses, cc_addresses,
	 subject, snippet, body_text, body_html, date,
	 is_read, is_flagged, has_attachments, is_draft, uid)
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

// message is one row to insert.
type message struct {
	id                           string
	folderID                     string
	from                         addr
	to                           []addr
	cc                           []addr
	subject, body, bodyHTML      string
	date                         time.Time
	read, flagged, attach, draft bool
}

func (m message) insert(ctx context.Context, stmt *sql.Stmt) error {
	_, err := stmt.ExecContext(ctx,
		m.id, accountID, m.folderID,
		fmt.Sprintf("<%s@example.com>", m.id), m.id,
		m.from.Email, nullable(m.from.Name), addrsJSON(m.to), nullableJSON(m.cc),
		nullable(m.subject), snippet(m.body), nullable(m.body), nullable(m.bodyHTML),
		m.date.UTC().Format("2006-01-02 15:04:05"),
		b2i(m.read), b2i(m.flagged), b2i(m.attach), b2i(m.draft), m.id[4:],
	)
	return err
}

func snippet(body string) any {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\n", " "))
	if body == "" {
		return nil
	}
	r := []rune(body)
	if len(r) > 140 {
		return string(r[:139]) + "…"
	}
	return body
}

func addrsJSON(as []addr) string {
	type ja struct {
		Name    string `json:"name,omitempty"`
		Address string `json:"address"`
	}
	out := make([]ja, len(as))
	for i, a := range as {
		out[i] = ja{Name: a.Name, Address: a.Email}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func nullableJSON(as []addr) any {
	if len(as) == 0 {
		return nil
	}
	return addrsJSON(as)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
