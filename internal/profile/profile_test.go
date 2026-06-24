package profile

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	meliadb "github.com/rubiojr/meliafts/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func buildDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "melia.db")
	w, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	require.NoError(t, meliadb.Apply(context.Background(), w))

	exec := func(q string, args ...any) {
		_, err := w.Exec(q, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO settings (key,value) VALUES ('schema_version','13')`)
	exec(`INSERT INTO accounts (id,name,email,type,incoming_host,incoming_port,incoming_security,outgoing_host,outgoing_port,outgoing_security,username,account_is_default)
		VALUES ('a','Me','me@x','imap','h',993,'ssl','h',465,'ssl','me@x',1)`)
	exec(`INSERT INTO folders (id,account_id,name,path,type) VALUES ('fi','a','Inbox','INBOX','inbox'), ('fs','a','Spam','Spam','spam')`)
	exec(`INSERT INTO messages (id,account_id,folder_id,message_id,from_address,to_addresses,subject,body_text,body_html,has_attachments,is_read,date) VALUES
		('m1','a','fi','<1@x>','bob@x','[{"address":"me@x"}]','Hello one','body one','<p>one</p>',0,0,'2024-01-01 09:00:00'),
		('m2','a','fi','<2@x>','bob@x','[{"address":"me@x"}]','Hello two','body two',NULL,0,1,'2024-02-01 09:00:00'),
		('m3','a','fi','<3@x>','bob@x','[{"address":"me@x"}]','Hello three','body three',NULL,1,1,'2024-03-01 09:00:00'),
		('m4','a','fs','<4@x>','win@x','[{"address":"me@x"}]','Spam four','body four',NULL,0,0,'2024-04-01 09:00:00')`)
	t.Cleanup(func() { w.Close() })
	return w
}

func TestCollect(t *testing.T) {
	p, err := Collect(buildDB(t))
	require.NoError(t, err)

	assert.Equal(t, 13, p.SchemaVersion)
	assert.Equal(t, 1, p.Accounts)

	require.Len(t, p.Folders, 2)
	byType := map[string]Folder{}
	for _, f := range p.Folders {
		byType[f.Type] = f
	}
	assert.Equal(t, Folder{Type: "inbox", Messages: 3, Unread: 1}, byType["inbox"])
	assert.Equal(t, Folder{Type: "spam", Messages: 1, Unread: 1}, byType["spam"])

	m := p.Messages
	assert.Equal(t, 4, m.Total)
	assert.Equal(t, 4, m.DistinctMessageID)
	assert.Equal(t, 2, m.DistinctSenders)
	assert.Equal(t, 2, m.Read)
	assert.Equal(t, 2, m.Unread)
	assert.Equal(t, 1, m.HasAttachments)
	assert.Equal(t, 1, m.WithHTML)
	assert.Equal(t, 4, m.WithText)
	assert.Equal(t, "2024-01-01 09:00:00", m.FirstDate)
	assert.Equal(t, "2024-04-01 09:00:00", m.LastDate)
	assert.Equal(t, map[string]int{"2024-01": 1, "2024-02": 1, "2024-03": 1, "2024-04": 1}, m.PerMonth)

	assert.GreaterOrEqual(t, p.TableRows["settings"], 1)
}

func TestCollectEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	w, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	require.NoError(t, meliadb.Apply(context.Background(), w))
	t.Cleanup(func() { w.Close() })

	p, err := Collect(w)
	require.NoError(t, err)
	assert.Zero(t, p.Messages.Total)
	assert.Empty(t, p.Folders)
}
