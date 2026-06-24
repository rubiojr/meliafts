package sampledb

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"

	meliadb "github.com/rubiojr/meliafts/db"
	"github.com/rubiojr/meliafts/internal/profile"
)

// BuildFromProfile creates a fresh database at path that reproduces the
// structure described by p — folder layout, message counts, date range, flag
// ratios and cross-folder ("All Mail") duplication — with synthetic content.
func BuildFromProfile(ctx context.Context, path string, p *profile.Profile, opts Options) error {
	db, err := openForWrite(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := GenerateFromProfile(ctx, db, p, opts); err != nil {
		return err
	}
	return db.Close()
}

// GenerateFromProfile applies the melia schema and fills db to match p. The
// schema applied is always the embedded one, so the generated database is
// stamped with the supported schema version regardless of p.SchemaVersion.
func GenerateFromProfile(ctx context.Context, db *sql.DB, p *profile.Profile, opts Options) error {
	opts = opts.withDefaults()
	if err := meliadb.Apply(ctx, db); err != nil {
		return err
	}
	if err := insertSettings(ctx, db); err != nil {
		return err
	}
	if err := insertAccount(ctx, db); err != nil {
		return err
	}

	plans := planFolders(p.Folders)
	if err := insertFolderPlans(ctx, db, plans); err != nil {
		return err
	}
	return insertProfileMessages(ctx, db, plans, p, opts)
}

// folderPlan is a folder to create plus how many messages it should hold.
type folderPlan struct {
	id, name, path, ptype string
	dbType                any // schema-valid type string, or nil
	count, unread         int
	archive               bool
}

func planFolders(fs []profile.Folder) []folderPlan {
	out := make([]folderPlan, len(fs))
	for i, f := range fs {
		name, path := folderNames(f.Type, i)
		out[i] = folderPlan{
			id:      fmt.Sprintf("f-%03d", i),
			name:    name,
			path:    path,
			ptype:   f.Type,
			dbType:  dbFolderType(f.Type),
			count:   f.Messages,
			unread:  f.Unread,
			archive: f.Type == "archive",
		}
	}
	return out
}

// dbFolderType maps a profile folder type to a value the embedded schema accepts
// (type IN inbox|sent|drafts|trash|spam|custom|NULL). "archive" (Gmail/Proton
// "All Mail") and unknown types become 'custom'; "(null)" becomes NULL.
func dbFolderType(ptype string) any {
	switch ptype {
	case "inbox", "sent", "drafts", "trash", "spam", "custom":
		return ptype
	case "(null)", "":
		return nil
	default: // archive and anything unexpected
		return "custom"
	}
}

func folderNames(ptype string, i int) (name, path string) {
	switch ptype {
	case "inbox":
		return "INBOX", "INBOX"
	case "sent":
		return "Sent", "Sent"
	case "drafts":
		return "Drafts", "Drafts"
	case "trash":
		return "Trash", "Trash"
	case "spam":
		return "Spam", "Spam"
	case "archive":
		return "All Mail", "All Mail"
	default:
		return fmt.Sprintf("Folder %d", i), fmt.Sprintf("Folder/%d", i)
	}
}

func insertFolderPlans(ctx context.Context, db *sql.DB, plans []folderPlan) error {
	for _, f := range plans {
		_, err := db.ExecContext(ctx,
			`INSERT INTO folders (id,account_id,name,path,type) VALUES (?,?,?,?,?)`,
			f.id, accountID, f.name, f.path, f.dbType)
		if err != nil {
			return fmt.Errorf("insert folder %s: %w", f.id, err)
		}
	}
	return nil
}

func insertProfileMessages(ctx context.Context, db *sql.DB, plans []folderPlan, p *profile.Profile, opts Options) error {
	rng := rand.New(rand.NewSource(opts.Seed))
	first, last := dateRange(p.Messages)
	r := ratios(p.Messages, archiveCount(plans))

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

	var pool []message // non-archive messages, the source for All Mail copies
	idx := 0
	for _, f := range ordered(plans) {
		for i := 0; i < f.count; i++ {
			idx++
			m := buildProfileMessage(rng, f, &pool, first, last, r, idx)
			m.read = i >= f.unread // the first f.unread rows of each folder are unread
			if err := m.insert(ctx, stmt); err != nil {
				return fmt.Errorf("insert %s: %w", m.id, err)
			}
		}
	}
	return tx.Commit()
}

// ordered returns the plans with archive folders last, so the duplication pool
// is populated from the real folders before any All Mail copies are made.
func ordered(plans []folderPlan) []folderPlan {
	out := make([]folderPlan, 0, len(plans))
	for _, f := range plans {
		if !f.archive {
			out = append(out, f)
		}
	}
	for _, f := range plans {
		if f.archive {
			out = append(out, f)
		}
	}
	return out
}

func buildProfileMessage(rng *rand.Rand, f folderPlan, pool *[]message, first, last time.Time, r msgRatios, idx int) message {
	id := fmt.Sprintf("msg-%06d", idx)
	// All Mail mirrors an existing message (same Message-ID) → dedup. The rate is
	// derived from the profile so the reproduced distinct_message_id matches.
	if f.archive && len(*pool) > 0 && rng.Float64() < r.dup {
		m := (*pool)[rng.Intn(len(*pool))]
		m.id = id
		m.folderID = f.id
		return m
	}

	m := freshContent(rng, f, r)
	m.id = id
	m.messageID = fmt.Sprintf("<gen-%06d@example.com>", idx)
	m.threadID = m.messageID
	m.folderID = f.id
	m.date = between(rng, first, last)
	m.flagged = rng.Float64() < r.flagged
	m.attach = rng.Float64() < r.attach
	if rng.Float64() < r.html {
		m.bodyHTML = htmlWrap(m.subject, m.snippet)
	}
	if !f.archive {
		*pool = append(*pool, m)
	}
	return m
}

func freshContent(rng *rand.Rand, f folderPlan, r msgRatios) message {
	var m message
	if f.ptype == "sent" || f.ptype == "drafts" {
		m.from, m.to = me, []addr{pick(rng, people)}
		m.draft = f.ptype == "drafts"
	} else {
		m.from, m.to = pick(rng, allSenders), []addr{me}
	}
	subject := pick(rng, allSubjects)
	if strings.Contains(subject, "%d") {
		subject = fmt.Sprintf(subject, 1000+rng.Intn(9000))
	}
	m.subject = subject
	// Every message has a snippet (the preview melia always stores); only the
	// with_text fraction also has the full body_text, which melia fetches lazily.
	text := pick(rng, allBodies)
	m.snippet = text
	if rng.Float64() < r.text {
		m.body = text
	}
	return m
}

// msgRatios are per-message probabilities derived from the profile.
type msgRatios struct{ flagged, attach, html, text, dup float64 }

func ratios(m profile.Messages, archive int) msgRatios {
	if m.Total == 0 {
		return msgRatios{}
	}
	t := float64(m.Total)
	return msgRatios{
		flagged: float64(m.Flagged) / t,
		attach:  float64(m.HasAttachments) / t,
		html:    float64(m.WithHTML) / t,
		text:    float64(m.WithText) / t,
		dup:     dupProbability(m, archive),
	}
}

// dupProbability is the chance an archive ("All Mail") message duplicates an
// existing one (sharing its Message-ID), chosen so the reproduced
// distinct_message_id matches the profile. Duplicates are only drawn from
// archive folders, so when the profile implies more duplicates than the archive
// can supply the probability is capped at 1.
func dupProbability(m profile.Messages, archive int) float64 {
	if archive == 0 {
		return 0
	}
	dupTarget := m.Total - m.DistinctMessageID
	if dupTarget <= 0 {
		return 0
	}
	if p := float64(dupTarget) / float64(archive); p < 1 {
		return p
	}
	return 1
}

// archiveCount is the total number of messages across archive folders.
func archiveCount(plans []folderPlan) int {
	n := 0
	for _, f := range plans {
		if f.archive {
			n += f.count
		}
	}
	return n
}

func dateRange(m profile.Messages) (first, last time.Time) {
	first, last = parseDate(m.FirstDate), parseDate(m.LastDate)
	if first.IsZero() || last.IsZero() || !first.Before(last) {
		last = time.Now()
		first = last.AddDate(-1, 0, 0)
	}
	return first, last
}

func parseDate(s string) time.Time {
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04:05.999999999-07:00", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func between(rng *rand.Rand, first, last time.Time) time.Time {
	span := last.Sub(first)
	if span <= 0 {
		return first
	}
	return first.Add(time.Duration(rng.Int63n(int64(span))))
}

// Flat content pools for profile generation, drawn from the category data.
var allSenders = func() []addr {
	var s []addr
	s = append(s, people...)
	s = append(s, newsletters...)
	s = append(s, services...)
	s = append(s, spammers...)
	return s
}()

var allSubjects, allBodies = func() ([]string, []string) {
	var subs, bods []string
	for _, c := range categories {
		subs = append(subs, c.subjects...)
		bods = append(bods, c.bodies...)
	}
	return subs, bods
}()
