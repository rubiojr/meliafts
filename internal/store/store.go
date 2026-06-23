// Package store provides read-only access to the melia database for searching
// and loading messages. It wraps the query layer so the CLI and the TUI share a
// single search implementation.
package store

import (
	"database/sql"
	"fmt"

	"github.com/rubiojr/meliafts/internal/db"
	"github.com/rubiojr/meliafts/internal/query"
)

// Message is a single mail message. The list fields are always populated by
// Search; the detail fields (ToAddresses, BodyText, BodyHTML) are only populated
// by Load.
type Message struct {
	ID             string `json:"id"`
	Date           string `json:"date"`
	IsRead         bool   `json:"is_read"`
	IsFlagged      bool   `json:"is_flagged"`
	HasAttachments bool   `json:"has_attachments"`
	FromName       string `json:"from_name"`
	FromAddress    string `json:"from_address"`
	Subject        string `json:"subject"`
	Snippet        string `json:"snippet"`
	ToAddresses    string `json:"to_addresses,omitempty"`
	BodyText       string `json:"body_text,omitempty"`
	BodyHTML       string `json:"body_html,omitempty"`
}

// Store is a read-only handle to the melia database.
type Store struct {
	db *sql.DB
}

// Open opens the melia database at path in read-only mode.
func Open(path string) (*Store, error) {
	d, err := db.OpenReadOnly(path)
	if err != nil {
		return nil, err
	}
	return &Store{db: d}, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// CheckSchema reports whether the open database's melia schema version is the
// one this build supports. It returns a *db.SchemaError on drift, nil otherwise.
func (s *Store) CheckSchema() error {
	return db.CheckSchema(s.db)
}

// Compile parses and compiles a Gmail-style query string, returning the SQL and
// bound arguments. It is exposed so callers (e.g. `--sql`/`--fts`) can inspect
// the generated query without executing it.
func (s *Store) Compile(queryStr string, limit int) (*query.Compiled, error) {
	q, err := query.Parse(queryStr)
	if err != nil {
		return nil, err
	}
	return q.Compile(query.Options{Limit: limit})
}

// Search runs a Gmail-style query and returns the matching messages with their
// list fields populated. A limit of 0 means no limit; offset skips that many
// rows (for paginated loading).
func (s *Store) Search(queryStr string, limit, offset int) ([]Message, error) {
	return s.search(queryStr, query.Options{Limit: limit, Offset: offset})
}

// SearchView is like Search but returns a deduplicated mailbox view: messages
// that appear in more than one folder (e.g. a Gmail/Proton "All Mail" copy) are
// collapsed to a single row per Message-ID. Unless includeSpam is set, every
// message whose Message-ID appears in a spam folder is hidden, even when a copy
// also lives in another folder. It backs the interactive TUI.
func (s *Store) SearchView(queryStr string, limit, offset int, includeSpam bool) ([]Message, error) {
	return s.search(queryStr, query.Options{
		Limit:    limit,
		Offset:   offset,
		Dedup:    true,
		HideSpam: !includeSpam,
	})
}

// search compiles queryStr with opts and returns the matching list rows.
func (s *Store) search(queryStr string, opts query.Options) ([]Message, error) {
	q, err := query.Parse(queryStr)
	if err != nil {
		return nil, err
	}
	compiled, err := q.Compile(opts)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(compiled.SQL, compiled.Args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var (
			m                            Message
			date, fromName, fromAddr     sql.NullString
			subject, snippet             sql.NullString
			isRead, isFlagged, hasAttach int
		)
		if err := rows.Scan(&m.ID, &date, &isRead, &isFlagged, &hasAttach, &fromName, &fromAddr, &subject, &snippet); err != nil {
			return nil, err
		}
		m.Date = date.String
		m.IsRead = isRead != 0
		m.IsFlagged = isFlagged != 0
		m.HasAttachments = hasAttach != 0
		m.FromName = fromName.String
		m.FromAddress = fromAddr.String
		m.Subject = subject.String
		m.Snippet = snippet.String
		out = append(out, m)
	}
	return out, rows.Err()
}

// Load fetches a single message by id including its body and recipients.
func (s *Store) Load(id string) (*Message, error) {
	const q = `SELECT id, date, is_read, is_flagged, has_attachments,
		from_name, from_address, to_addresses, subject, snippet, body_text, body_html
		FROM messages WHERE id = ?`

	var (
		m                                 Message
		date, fromName, fromAddr, toAddr  sql.NullString
		subject, snippet, bodyTxt, bodyHT sql.NullString
		isRead, isFlagged, hasAttach      int
	)
	err := s.db.QueryRow(q, id).Scan(
		&m.ID, &date, &isRead, &isFlagged, &hasAttach,
		&fromName, &fromAddr, &toAddr, &subject, &snippet, &bodyTxt, &bodyHT,
	)
	if err != nil {
		return nil, fmt.Errorf("load message %q: %w", id, err)
	}

	m.Date = date.String
	m.IsRead = isRead != 0
	m.IsFlagged = isFlagged != 0
	m.HasAttachments = hasAttach != 0
	m.FromName = fromName.String
	m.FromAddress = fromAddr.String
	m.ToAddresses = toAddr.String
	m.Subject = subject.String
	m.Snippet = snippet.String
	m.BodyText = bodyTxt.String
	m.BodyHTML = bodyHT.String
	return &m, nil
}
