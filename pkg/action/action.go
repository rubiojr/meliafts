// Package action is the public SDK for meliafts actions: the small programs and
// scripts meliafts runs when new mail arrives (see docs/actions.md). It defines
// the action contract — the MELIAFTS_* environment variables and the JSON object
// delivered on stdin — once, so both sides agree:
//
//   - Action authors call [Handle] (or [Read]) to receive a typed [Event] for
//     each new message instead of parsing the environment and stdin by hand:
//
//     package main
//
//     import "github.com/rubiojr/meliafts/pkg/action"
//
//     func main() {
//     action.Handle(func(ev action.Event) error {
//     if ev.Message.IsRead {
//     return nil
//     }
//     return notify(ev.Message.Subject)
//     })
//     }
//
//   - Host/embedding tools use [Event.Env] and [Event.JSON] to produce exactly
//     the payload meliafts itself sends.
//
// The package depends only on the standard library.
package action

import "encoding/json"

// Environment variable names meliafts sets for each action invocation.
const (
	EnvEvent          = "MELIAFTS_EVENT"
	EnvDB             = "MELIAFTS_DB"
	EnvQuery          = "MELIAFTS_QUERY"
	EnvID             = "MELIAFTS_ID"
	EnvDate           = "MELIAFTS_DATE"
	EnvSubject        = "MELIAFTS_SUBJECT"
	EnvFromName       = "MELIAFTS_FROM_NAME"
	EnvFromAddress    = "MELIAFTS_FROM_ADDRESS"
	EnvSnippet        = "MELIAFTS_SNIPPET"
	EnvUnread         = "MELIAFTS_UNREAD"
	EnvFlagged        = "MELIAFTS_FLAGGED"
	EnvHasAttachments = "MELIAFTS_HAS_ATTACHMENTS"
)

// EventNew is the event name for a newly-seen message. It is currently the only
// event meliafts emits.
const EventNew = "new-message"

// Message is the mail message that triggered an action. Its JSON encoding is
// exactly the object meliafts pipes to the action on stdin; the body and
// recipient fields are only present when the host includes them.
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

// Event is the full context for one action invocation.
type Event struct {
	Name    string  // the event name, e.g. EventNew
	Query   string  // the query meliafts is watching
	DBPath  string  // path to the read-only melia database
	Message Message // the message that triggered the event
}

// Env renders the event as a "KEY=value" environment slice suitable for
// exec.Cmd.Env. It is the producer counterpart of [Read].
func (e Event) Env() []string {
	m := e.Message
	return []string{
		EnvEvent + "=" + e.Name,
		EnvDB + "=" + e.DBPath,
		EnvQuery + "=" + e.Query,
		EnvID + "=" + m.ID,
		EnvDate + "=" + m.Date,
		EnvSubject + "=" + m.Subject,
		EnvFromName + "=" + m.FromName,
		EnvFromAddress + "=" + m.FromAddress,
		EnvSnippet + "=" + m.Snippet,
		EnvUnread + "=" + bit(!m.IsRead),
		EnvFlagged + "=" + bit(m.IsFlagged),
		EnvHasAttachments + "=" + bit(m.HasAttachments),
	}
}

// JSON renders the triggering message as the stdin payload meliafts sends. It
// never fails: an unmarshalable message yields "{}".
func (e Event) JSON() []byte {
	b, err := json.Marshal(e.Message)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func bit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
