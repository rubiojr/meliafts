package action

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleEvent() Event {
	return Event{
		Name:   EventNew,
		Query:  "unread:",
		DBPath: "/db/melia.db",
		Message: Message{
			ID: "m1", Date: "2026-01-01 10:00:00",
			IsRead: false, IsFlagged: true, HasAttachments: false,
			FromName: "Amazon", FromAddress: "a@b.com",
			Subject: "Hi & welcome", Snippet: "preview",
		},
	}
}

func TestEventEnv(t *testing.T) {
	env := sampleEvent().Env()
	want := []string{
		"MELIAFTS_EVENT=new-message",
		"MELIAFTS_DB=/db/melia.db",
		"MELIAFTS_QUERY=unread:",
		"MELIAFTS_ID=m1",
		"MELIAFTS_SUBJECT=Hi & welcome",
		"MELIAFTS_FROM_ADDRESS=a@b.com",
		"MELIAFTS_UNREAD=1",
		"MELIAFTS_FLAGGED=1",
		"MELIAFTS_HAS_ATTACHMENTS=0",
	}
	for _, w := range want {
		assert.Contains(t, env, w)
	}
}

func TestEventJSON(t *testing.T) {
	b := sampleEvent().JSON()
	s := string(b)
	assert.Contains(t, s, `"id":"m1"`)
	assert.Contains(t, s, `"from_address":"a@b.com"`)
	// Optional body/recipient fields are omitted when empty.
	assert.NotContains(t, s, "body_text")
	assert.NotContains(t, s, "to_addresses")
}

// envGetter turns an Env() slice into a getenv-style lookup.
func envGetter(pairs []string) func(string) string {
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		if k, v, ok := strings.Cut(p, "="); ok {
			m[k] = v
		}
	}
	return func(k string) string { return m[k] }
}

func TestDecodeFromStdin(t *testing.T) {
	in := sampleEvent()
	got, err := decode(envGetter(in.Env()), bytes.NewReader(in.JSON()))
	require.NoError(t, err)
	assert.Equal(t, in, got, "the message should round-trip via stdin JSON")
}

func TestDecodeFromEnvWhenStdinEmpty(t *testing.T) {
	in := sampleEvent()
	got, err := decode(envGetter(in.Env()), strings.NewReader("  \n"))
	require.NoError(t, err)
	// Reconstructed from the environment (which carries every scalar field).
	assert.Equal(t, in, got)
}

func TestDecodeBadJSON(t *testing.T) {
	_, err := decode(func(string) string { return "" }, strings.NewReader("{not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode message")
}

func TestDecodeUnreadMapping(t *testing.T) {
	// MELIAFTS_UNREAD=1 means the message is *not* read.
	get := func(k string) string {
		if k == EnvUnread {
			return "1"
		}
		return ""
	}
	ev, err := decode(get, strings.NewReader(""))
	require.NoError(t, err)
	assert.False(t, ev.Message.IsRead)
}
