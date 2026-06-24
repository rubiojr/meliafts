// Package profile extracts an aggregate, content-free statistical profile of a
// melia database — message counts, folder layout, date range, flag ratios and
// the cross-folder duplication factor — without reading any private content
// (no addresses, subjects or bodies). The profile is safe to share and can drive
// synthetic fixture generation (see internal/sampledb).
package profile

import (
	"database/sql"

	"github.com/rubiojr/meliafts/internal/db"
)

// Profile is the structural summary of a melia database.
type Profile struct {
	SchemaVersion int            `json:"schema_version"`
	Accounts      int            `json:"accounts"`
	Folders       []Folder       `json:"folders"`
	Messages      Messages       `json:"messages"`
	TableRows     map[string]int `json:"table_rows"`
}

// Folder summarises one folder by type and size (no name or path).
type Folder struct {
	Type     string `json:"type"`
	Messages int    `json:"messages"`
	Unread   int    `json:"unread"`
}

// Messages holds the message-level aggregates.
type Messages struct {
	Total             int            `json:"total"`
	DistinctMessageID int            `json:"distinct_message_id"`
	DistinctThreadID  int            `json:"distinct_thread_id"`
	DistinctSenders   int            `json:"distinct_senders"`
	FirstDate         string         `json:"first_date"`
	LastDate          string         `json:"last_date"`
	Read              int            `json:"read"`
	Unread            int            `json:"unread"`
	Flagged           int            `json:"flagged"`
	HasAttachments    int            `json:"has_attachments"`
	WithHTML          int            `json:"with_html"`
	WithText          int            `json:"with_text"`
	Drafts            int            `json:"drafts"`
	PerMonth          map[string]int `json:"per_month"`
	Subject           Lengths        `json:"subject_len"`
	BodyText          Lengths        `json:"body_text_len"`
	Snippet           Lengths        `json:"snippet_len"`
	Recipients        Recipients     `json:"recipients"`
}

// Lengths is a min/max/avg summary of a text column's length.
type Lengths struct {
	Min int     `json:"min"`
	Max int     `json:"max"`
	Avg float64 `json:"avg"`
}

// Recipients holds the average number of to/cc/bcc recipients per message.
type Recipients struct {
	ToAvg  float64 `json:"to_avg"`
	CcAvg  float64 `json:"cc_avg"`
	BccAvg float64 `json:"bcc_avg"`
}

// otherTables are counted (rows only, no content) for completeness.
var otherTables = []string{
	"contacts", "blocked_senders", "trusted_senders", "sender_stats",
	"trust_activity_log", "outbox_messages", "deferred_sync_queue",
	"search_history", "inbox_rules", "aliases", "account_canonical_folders",
	"settings",
}

// Collect reads d (read-only is fine) and returns its structural profile.
func Collect(d *sql.DB) (*Profile, error) {
	p := &Profile{}
	if v, ok := db.SchemaVersion(d); ok {
		p.SchemaVersion = v
	}
	p.Accounts = scalar(d, "SELECT COUNT(*) FROM accounts")

	folders, err := collectFolders(d)
	if err != nil {
		return nil, err
	}
	p.Folders = folders

	msgs, err := collectMessages(d)
	if err != nil {
		return nil, err
	}
	p.Messages = msgs

	p.TableRows = collectTableRows(d)
	return p, nil
}

func collectFolders(d *sql.DB) ([]Folder, error) {
	rows, err := d.Query(`SELECT COALESCE(f.type,'(null)'),
		COUNT(m.id),
		COALESCE(SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END), 0)
		FROM folders f LEFT JOIN messages m ON m.folder_id = f.id
		GROUP BY f.id ORDER BY 2 DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Folder
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.Type, &f.Messages, &f.Unread); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func collectMessages(d *sql.DB) (Messages, error) {
	var m Messages
	var read int
	const q = `SELECT
		COUNT(*),
		COUNT(DISTINCT NULLIF(message_id,'')),
		COUNT(DISTINCT NULLIF(thread_id,'')),
		COUNT(DISTINCT from_address),
		COALESCE(MIN(date),''), COALESCE(MAX(date),''),
		COALESCE(SUM(is_read),0),
		COALESCE(SUM(is_flagged),0),
		COALESCE(SUM(has_attachments),0),
		COALESCE(SUM(CASE WHEN body_html IS NOT NULL AND body_html <> '' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN body_text IS NOT NULL AND body_text <> '' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(COALESCE(is_draft,0)),0),
		COALESCE(MIN(LENGTH(subject)),0), COALESCE(MAX(LENGTH(subject)),0), COALESCE(AVG(LENGTH(subject)),0),
		COALESCE(MIN(LENGTH(body_text)),0), COALESCE(MAX(LENGTH(body_text)),0), COALESCE(AVG(LENGTH(body_text)),0),
		COALESCE(MIN(LENGTH(snippet)),0), COALESCE(MAX(LENGTH(snippet)),0), COALESCE(AVG(LENGTH(snippet)),0),
		COALESCE(AVG(CASE WHEN json_valid(to_addresses)  THEN json_array_length(to_addresses)  ELSE 0 END),0),
		COALESCE(AVG(CASE WHEN json_valid(cc_addresses)  THEN json_array_length(cc_addresses)  ELSE 0 END),0),
		COALESCE(AVG(CASE WHEN json_valid(bcc_addresses) THEN json_array_length(bcc_addresses) ELSE 0 END),0)
		FROM messages`
	err := d.QueryRow(q).Scan(
		&m.Total, &m.DistinctMessageID, &m.DistinctThreadID, &m.DistinctSenders,
		&m.FirstDate, &m.LastDate,
		&read, &m.Flagged, &m.HasAttachments, &m.WithHTML, &m.WithText, &m.Drafts,
		&m.Subject.Min, &m.Subject.Max, &m.Subject.Avg,
		&m.BodyText.Min, &m.BodyText.Max, &m.BodyText.Avg,
		&m.Snippet.Min, &m.Snippet.Max, &m.Snippet.Avg,
		&m.Recipients.ToAvg, &m.Recipients.CcAvg, &m.Recipients.BccAvg,
	)
	if err != nil {
		return m, err
	}
	m.Read = read
	m.Unread = m.Total - read

	pm, err := perMonth(d)
	if err != nil {
		return m, err
	}
	m.PerMonth = pm
	return m, nil
}

func perMonth(d *sql.DB) (map[string]int, error) {
	rows, err := d.Query(`SELECT substr(date,1,7), COUNT(*) FROM messages GROUP BY 1 ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var ym string
		var n int
		if err := rows.Scan(&ym, &n); err != nil {
			return nil, err
		}
		out[ym] = n
	}
	return out, rows.Err()
}

func collectTableRows(d *sql.DB) map[string]int {
	out := map[string]int{}
	for _, t := range otherTables {
		var n int
		// Table names come from a fixed allowlist, never user input.
		if err := d.QueryRow("SELECT COUNT(*) FROM " + t).Scan(&n); err == nil {
			out[t] = n
		}
	}
	return out
}

func scalar(d *sql.DB, q string) int {
	var n int
	_ = d.QueryRow(q).Scan(&n)
	return n
}
