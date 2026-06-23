package meliadb

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestApplyAndTriggers(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	require.NoError(t, Apply(ctx, db))

	// Seed the minimal referenced rows.
	_, err = db.Exec(`INSERT INTO accounts
		(id,name,email,type,incoming_host,incoming_port,incoming_security,outgoing_host,outgoing_port,outgoing_security,username)
		VALUES ('a1','Me','me@example.com','imap','imap.example.com',993,'ssl','smtp.example.com',465,'ssl','me')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO folders (id,account_id,name,path,type) VALUES ('f1','a1','Inbox','INBOX','inbox')`)
	require.NoError(t, err)

	// Inserting a message must auto-populate messages_fts via the trigger,
	// including to_text built from the JSON recipients.
	_, err = db.Exec(`INSERT INTO messages
		(id,account_id,folder_id,from_address,from_name,to_addresses,subject,snippet,body_text,date,is_read)
		VALUES ('m1','a1','f1','bob@acme.com','Bob Smith',
		'[{"name":"Alice","address":"alice@acme.com"}]',
		'Invoice 2024','your invoice','please pay the kubernetes invoice','2024-03-01 10:00:00',0)`)
	require.NoError(t, err)

	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM messages_fts WHERE messages_fts MATCH 'subject:invoice'`).Scan(&n))
	assert.Equal(t, 1, n)

	require.NoError(t, db.QueryRow(`SELECT count(*) FROM messages_fts WHERE messages_fts MATCH 'to_text:alice'`).Scan(&n))
	assert.Equal(t, 1, n, "to_text should be built from recipients by the trigger")

	require.NoError(t, db.QueryRow(`SELECT count(*) FROM messages_fts WHERE messages_fts MATCH 'body_text:kubernetes'`).Scan(&n))
	assert.Equal(t, 1, n)

	// The unread trigger should have bumped the folder counter.
	var unread int
	require.NoError(t, db.QueryRow(`SELECT unread_count FROM folders WHERE id='f1'`).Scan(&unread))
	assert.Equal(t, 1, unread)
}
